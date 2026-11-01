package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/expvintl/scopex/internal/config"
	"github.com/expvintl/scopex/internal/job"
	"github.com/expvintl/scopex/internal/store"
)

// 会话相关测试需要真实 MongoDB，未设置 SCOPEX_TEST_MONGO 时跳过：
//
//	SCOPEX_TEST_MONGO=mongodb://127.0.0.1:27017 go test ./internal/api/
//
// 覆盖的是这样一个真实事故：登录成功后把数据库整个删掉重新初始化并设置新密码，
// 旧设备上残留的令牌依然能读写后台，且带着管理员实权。

const testAdminPass = "TestPass123456"

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func testConfig() config.Config {
	return config.Config{
		Listen:    ":0",
		MongoURI:  "mongodb://127.0.0.1:27017",
		MongoDB:   "scopex_test",
		JWTSecret: "",
		TokenTTL:  time.Hour,
		AdminUser: "admin",
		AdminPass: testAdminPass,
		MaxExport: 1000,
		Scanner: config.ScannerConfig{
			MaxRate:      1000,
			MaxTargets:   1024,
			ProbeTimeout: time.Second,
		},
	}
}

// setup 建一个独立库，并完全照 cmd/server/main.go 的顺序初始化鉴权数据。
func setup(t *testing.T) (*store.Store, config.Config) {
	t.Helper()
	uri := os.Getenv("SCOPEX_TEST_MONGO")
	if uri == "" {
		t.Skip("未设置 SCOPEX_TEST_MONGO，跳过需要 MongoDB 的会话测试")
	}
	ctx := context.Background()
	name := fmt.Sprintf("scopex_test_%d", time.Now().UnixNano())
	cfg := testConfig()
	cfg.MongoURI = uri
	cfg.MongoDB = name

	st, err := store.Open(ctx, uri, name)
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	t.Cleanup(func() {
		_ = st.DB.Drop(context.Background())
		_ = st.Close(context.Background())
	})
	if err := bootstrapAuth(ctx, st, &cfg); err != nil {
		t.Fatalf("初始化鉴权数据失败: %v", err)
	}
	return st, cfg
}

// bootstrapAuth 与 main.go 保持同一套初始化顺序：签名密钥 → 初始管理员 → 补齐会话标识。
func bootstrapAuth(ctx context.Context, st *store.Store, cfg *config.Config) error {
	secret, _, err := st.LoadOrCreateAuthSecret(ctx, cfg.JWTSecret)
	if err != nil {
		return err
	}
	cfg.JWTSecret = secret
	if _, _, err := st.EnsureAdmin(ctx, cfg.AdminUser, cfg.AdminPass); err != nil {
		return err
	}
	if _, err := st.BackfillSessionNonces(ctx); err != nil {
		return err
	}
	return nil
}

func newTestServer(t *testing.T, st *store.Store, cfg config.Config) http.Handler {
	t.Helper()
	engine := job.NewEngine(cfg, st, nil, nil, testLogger())
	return New(cfg, st, engine, nil, nil, testLogger(), nil).Router()
}

func call(t *testing.T, h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func login(t *testing.T, h http.Handler, password string) string {
	t.Helper()
	w := call(t, h, http.MethodPost, "/api/auth/login", "",
		fmt.Sprintf(`{"username":"admin","password":%q}`, password))
	if w.Code != http.StatusOK {
		t.Fatalf("登录失败: %d %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	const marker = `"token":"`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("登录响应缺少 token: %s", body)
	}
	rest := body[i+len(marker):]
	return rest[:strings.Index(rest, `"`)]
}

type endpoint struct {
	method string
	path   string
	body   string
}

// authCalls 是登录即可访问的只读端点，viewer 也应该能通过。
var authCalls = []endpoint{
	{http.MethodGet, "/api/auth/me", ""},
	{http.MethodGet, "/api/tasks", ""},
}

// adminCalls 是一组“只有管理员才能通过”的端点，覆盖发起探测、写、删和用户管理。
var adminCalls = []endpoint{
	{http.MethodPost, "/api/tasks", `{"name":"probe","targets":["192.168.6.0/30"],"ports":"80"}`},
	{http.MethodDelete, "/api/tasks", ""},
	{http.MethodPost, "/api/ips/192.168.6.1/refresh", ""},
	{http.MethodGet, "/api/users", ""},
}

func allCalls() []endpoint {
	return append(append([]endpoint{}, authCalls...), adminCalls...)
}

func assertUnauthorized(t *testing.T, h http.Handler, token, scenario string) {
	t.Helper()
	for _, c := range allCalls() {
		w := call(t, h, c.method, c.path, token, c.body)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: %s %s 期望 401，实际 %d: %s",
				scenario, c.method, c.path, w.Code, strings.TrimSpace(w.Body.String()))
		}
	}
}

// TestLegacyTokenDiesAfterDatabaseReset 复现原始事故：删库重建账号后，旧令牌必须立刻作废。
func TestLegacyTokenDiesAfterDatabaseReset(t *testing.T) {
	ctx := context.Background()
	st, cfg := setup(t)
	h := newTestServer(t, st, cfg)

	token := login(t, h, testAdminPass)
	if w := call(t, h, http.MethodGet, "/api/auth/me", token, ""); w.Code != http.StatusOK {
		t.Fatalf("登录后访问失败: %d %s", w.Code, w.Body.String())
	}
	oldUser, err := st.FindUser(ctx, "admin")
	if err != nil {
		t.Fatalf("读取管理员失败: %v", err)
	}

	// 把数据库整个删掉，然后按首次启动的流程重新初始化，并设置新密码。
	if err := st.DB.Drop(ctx); err != nil {
		t.Fatalf("删除测试库失败: %v", err)
	}
	if err := bootstrapAuth(ctx, st, &cfg); err != nil {
		t.Fatalf("重新初始化失败: %v", err)
	}
	newUser, err := st.FindUser(ctx, "admin")
	if err != nil {
		t.Fatalf("重建后读取管理员失败: %v", err)
	}
	if newUser.ID == oldUser.ID {
		t.Fatal("重建后应当是新的用户记录")
	}

	// 情形 A：仍在运行的旧实例（内存里还是换密钥之前的那个），只能靠“回库核对”挡住旧令牌。
	stillRunning := newTestServer(t, st, cfg)
	assertUnauthorized(t, stillRunning, token, "旧实例收到删库前的令牌")

	// 情形 B：重启后的新实例，密钥已更换，签名校验就过不去。
	assertUnauthorized(t, newTestServer(t, st, cfg), token, "重启后的实例收到删库前的令牌")

	// 新账号重新登录必须正常可用。
	fresh := login(t, h, testAdminPass)
	if w := call(t, h, http.MethodGet, "/api/auth/me", fresh, ""); w.Code != http.StatusOK {
		t.Fatalf("新令牌不可用: %d %s", w.Code, w.Body.String())
	}
}

// TestForgedTokenWithPublicSecretRejected 旧版本默认密钥写在仓库里，任何人都能据此伪造管理员令牌。
func TestForgedTokenWithPublicSecretRejected(t *testing.T) {
	st, cfg := setup(t)
	h := newTestServer(t, st, cfg)
	if cfg.JWTSecret == "scopex-dev-secret-change-me" {
		t.Fatal("签名密钥不能沿用仓库里的公开默认值")
	}
	claims := jwt.MapClaims{
		"sub": "000000000000000000000000",
		"iss": "scopex",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	forged, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte("scopex-dev-secret-change-me"))
	if err != nil {
		t.Fatalf("构造伪造令牌失败: %v", err)
	}
	assertUnauthorized(t, h, forged, "公开默认密钥伪造的令牌")
}

// TestDeletedUserTokenRejected 删号之后，旧令牌不能再读任何接口。
func TestDeletedUserTokenRejected(t *testing.T) {
	ctx := context.Background()
	st, cfg := setup(t)
	h := newTestServer(t, st, cfg)
	token := login(t, h, testAdminPass)

	if _, err := st.Users.DeleteOne(ctx, bson.M{"username": "admin"}); err != nil {
		t.Fatalf("删除账号失败: %v", err)
	}
	assertUnauthorized(t, h, token, "账号已删除")
}

// TestRoleDowngradeTakesEffectImmediately 令牌里不再携带角色，降权必须立刻生效。
func TestRoleDowngradeTakesEffectImmediately(t *testing.T) {
	ctx := context.Background()
	st, cfg := setup(t)
	h := newTestServer(t, st, cfg)
	token := login(t, h, testAdminPass)

	if _, err := st.Users.UpdateOne(ctx, bson.M{"username": "admin"},
		bson.M{"$set": bson.M{"role": "viewer"}}); err != nil {
		t.Fatalf("降权失败: %v", err)
	}
	for _, c := range adminCalls {
		w := call(t, h, c.method, c.path, token, c.body)
		if w.Code != http.StatusForbidden {
			t.Errorf("降权后 %s %s 期望 403，实际 %d: %s",
				c.method, c.path, w.Code, strings.TrimSpace(w.Body.String()))
		}
	}
	// 只读接口仍应可用。
	for _, c := range authCalls {
		if w := call(t, h, c.method, c.path, token, c.body); w.Code != http.StatusOK {
			t.Fatalf("降权后 %s %s 应当可用: %d", c.method, c.path, w.Code)
		}
	}
}

// TestChangePasswordRotatesSessions 改密码要让其它设备下线，当前设备拿到新令牌继续用。
func TestChangePasswordRotatesSessions(t *testing.T) {
	ctx := context.Background()
	st, cfg := setup(t)
	h := newTestServer(t, st, cfg)
	old := login(t, h, testAdminPass)

	w := call(t, h, http.MethodPost, "/api/auth/password", old,
		`{"oldPassword":"`+testAdminPass+`","newPassword":"AnotherPass98765"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("修改密码失败: %d %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	const marker = `"token":"`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("修改密码的响应应带回新令牌: %s", body)
	}
	rest := body[i+len(marker):]
	renewed := rest[:strings.Index(rest, `"`)]

	assertUnauthorized(t, h, old, "改密码前的令牌")
	if w := call(t, h, http.MethodGet, "/api/auth/me", renewed, ""); w.Code != http.StatusOK {
		t.Fatalf("改密码后返回的新令牌不可用: %d %s", w.Code, w.Body.String())
	}
	// 用旧密码不应再能登录。
	if w := call(t, h, http.MethodPost, "/api/auth/login", "",
		`{"username":"admin","password":"`+testAdminPass+`"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("旧密码仍然可以登录: %d", w.Code)
	}
	u, err := st.FindUser(ctx, "admin")
	if err != nil {
		t.Fatalf("读取账号失败: %v", err)
	}
	if u.SessionNonce == "" {
		t.Fatal("会话标识不能为空")
	}
}

// TestAPITokenStillWorks API Key 走的是数据库查询，不依赖会话标识。
func TestAPITokenStillWorks(t *testing.T) {
	ctx := context.Background()
	st, cfg := setup(t)
	h := newTestServer(t, st, cfg)
	u, err := st.FindUser(ctx, "admin")
	if err != nil {
		t.Fatalf("读取管理员失败: %v", err)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	r.Header.Set("X-API-Key", u.APIKey)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("API Key 认证失败: %d %s", w.Code, w.Body.String())
	}
}
