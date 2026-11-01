package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"

	"github.com/expvintl/scopex/internal/model"
	"github.com/expvintl/scopex/internal/util"
)

const ctxUserKey = "scopex.user"

type sessionClaims struct {
	Username string `json:"name"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func (s *Server) issueToken(u *model.User) (string, time.Time, error) {
	now := time.Now()
	expire := now.Add(s.cfg.TokenTTL)
	claims := sessionClaims{
		Username: u.Username,
		Role:     u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.Hex(),
			Issuer:    "scopex",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expire),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTSecret))
	return signed, expire, err
}

func (s *Server) parseToken(raw string) (*sessionClaims, error) {
	claims := &sessionClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("非法的签名算法")
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("令牌无效")
	}
	return claims, nil
}

func (s *Server) userFromToken(raw string) *model.User {
	claims, err := s.parseToken(raw)
	if err != nil {
		return nil
	}
	oid, err := primitive.ObjectIDFromHex(claims.Subject)
	if err != nil {
		return nil
	}
	return &model.User{ID: oid, Username: claims.Username, Role: claims.Role}
}

func (s *Server) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
			if u := s.userFromToken(strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))); u != nil {
				c.Set(ctxUserKey, u)
				c.Next()
				return
			}
		}
		if key := strings.TrimSpace(c.GetHeader("X-API-Key")); key != "" {
			if u, err := s.store.FindUserByAPIKey(c.Request.Context(), key); err == nil {
				c.Set(ctxUserKey, u)
				c.Next()
				return
			}
		}
		// 图片、导出等无法附加请求头的场景允许通过查询参数传递令牌。
		if c.Request.Method == http.MethodGet {
			if tok := strings.TrimSpace(c.Query("token")); tok != "" {
				if u := s.userFromToken(tok); u != nil {
					c.Set(ctxUserKey, u)
					c.Next()
					return
				}
			}
		}
		fail(c, http.StatusUnauthorized, "未认证或登录已过期")
	}
}

func (s *Server) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if u := currentUser(c); u == nil || u.Role != "admin" {
			fail(c, http.StatusForbidden, "需要管理员权限")
			return
		}
		c.Next()
	}
}

func currentUser(c *gin.Context) *model.User {
	if v, exists := c.Get(ctxUserKey); exists {
		if u, ok := v.(*model.User); ok {
			return u
		}
	}
	return nil
}

func (s *Server) login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请输入用户名和密码")
		return
	}
	u, err := s.store.FindUser(c.Request.Context(), strings.TrimSpace(req.Username))
	if err != nil || !s.store.CheckPassword(u, req.Password) {
		fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	token, expire, err := s.issueToken(u)
	if err != nil {
		fail(c, http.StatusInternalServerError, "签发令牌失败")
		return
	}
	s.store.TouchLogin(c.Request.Context(), u.Username)
	ok(c, gin.H{"token": token, "expiresAt": expire, "user": u})
}

func (s *Server) me(c *gin.Context) {
	u := currentUser(c)
	fresh, err := s.store.FindUser(c.Request.Context(), u.Username)
	if err != nil {
		fail(c, http.StatusUnauthorized, "用户不存在")
		return
	}
	ok(c, fresh)
}

func (s *Server) changePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数不完整")
		return
	}
	if len(req.NewPassword) < 8 {
		fail(c, http.StatusBadRequest, "新密码至少 8 位")
		return
	}
	u := currentUser(c)
	stored, err := s.store.FindUser(c.Request.Context(), u.Username)
	if err != nil || !s.store.CheckPassword(stored, req.OldPassword) {
		fail(c, http.StatusBadRequest, "原密码不正确")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		fail(c, http.StatusInternalServerError, "加密失败")
		return
	}
	if err := s.store.UpdatePassword(c.Request.Context(), u.Username, string(hash)); err != nil {
		fail(c, http.StatusInternalServerError, "保存失败")
		return
	}
	ok(c, gin.H{"message": "密码已更新"})
}

func (s *Server) rotateAPIKey(c *gin.Context) {
	u := currentUser(c)
	key := util.RandHex(24)
	if err := s.store.SetAPIKey(c.Request.Context(), u.Username, key); err != nil {
		fail(c, http.StatusInternalServerError, "生成 API Key 失败")
		return
	}
	ok(c, gin.H{"apiKey": key})
}
