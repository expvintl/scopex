package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
	"github.com/expvintl/scopex/internal/model"
	"github.com/expvintl/scopex/internal/util"
)

// EnsureAdmin 在没有用户时创建初始管理员，返回（明文密码, 是否新建）。
func (s *Store) EnsureAdmin(ctx context.Context, username, password string) (string, bool, error) {
	n, err := s.Users.CountDocuments(ctx, bson.M{})
	if err != nil {
		return "", false, err
	}
	if n > 0 {
		return "", false, nil
	}
	generated := false
	if password == "" {
		password = util.RandPassword(14)
		generated = true
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", false, err
	}
	u := model.User{
		Username:     username,
		PasswordHash: string(hash),
		Role:         "admin",
		APIKey:       util.RandHex(24),
		CreatedAt:    time.Now(),
	}
	if _, err := s.Users.InsertOne(ctx, u); err != nil {
		return "", false, err
	}
	if generated {
		return password, true, nil
	}
	return "", true, nil
}

func (s *Store) FindUser(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	if err := s.Users.FindOne(ctx, bson.M{"username": username}).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) FindUserByAPIKey(ctx context.Context, key string) (*model.User, error) {
	var u model.User
	if err := s.Users.FindOne(ctx, bson.M{"api_key": key}).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) CheckPassword(u *model.User, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
}

// UpdatePassword 更新用户密码哈希。
func (s *Store) UpdatePassword(ctx context.Context, username, hash string) error {
	_, err := s.Users.UpdateOne(ctx, bson.M{"username": username}, bson.M{"$set": bson.M{"password_hash": hash}})
	return err
}

// SetAPIKey 重新生成并保存用户的 API Key。
func (s *Store) SetAPIKey(ctx context.Context, username, key string) error {
	_, err := s.Users.UpdateOne(ctx, bson.M{"username": username}, bson.M{"$set": bson.M{"api_key": key}})
	return err
}

func (s *Store) TouchLogin(ctx context.Context, username string) {
	now := time.Now()
	_, _ = s.Users.UpdateOne(ctx, bson.M{"username": username}, bson.M{"$set": bson.M{"last_login_at": now}})
}

func (s *Store) ListUsers(ctx context.Context) ([]model.User, error) {
	cur, err := s.Users.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var users []model.User
	if err := cur.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (s *Store) CreateUser(ctx context.Context, username, password, role string) (*model.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := model.User{
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
		APIKey:       util.RandHex(24),
		CreatedAt:    time.Now(),
	}
	res, err := s.Users.InsertOne(ctx, u)
	if err != nil {
		return nil, err
	}
	u.ID = res.InsertedID.(primitive.ObjectID)
	return &u, nil
}
