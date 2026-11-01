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

// newSessionNonce 生成一个随机会话标识。
func newSessionNonce() string { return util.RandHex(16) }

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
		SessionNonce: newSessionNonce(),
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

// FindUserByID 按 _id 读取用户。令牌里的 subject 是创建账号时的 _id，
// 删库重建后同一个用户名会得到新的 _id，这里查不到就说明令牌对应的账号已不存在。
func (s *Store) FindUserByID(ctx context.Context, id primitive.ObjectID) (*model.User, error) {
	var u model.User
	if err := s.Users.FindOne(ctx, bson.M{"_id": id}).Decode(&u); err != nil {
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
	if u == nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
}

// RotateSessionNonce 换掉会话标识，返回新值；该账号此前签发的令牌随即失效。
func (s *Store) RotateSessionNonce(ctx context.Context, username string) (string, error) {
	nonce := newSessionNonce()
	_, err := s.Users.UpdateOne(ctx, bson.M{"username": username},
		bson.M{"$set": bson.M{"session_nonce": nonce}})
	if err != nil {
		return "", err
	}
	return nonce, nil
}

// UpdatePassword 更新用户密码哈希，同时旋转会话标识，返回新的会话标识。
func (s *Store) UpdatePassword(ctx context.Context, username, hash string) (string, error) {
	nonce := newSessionNonce()
	_, err := s.Users.UpdateOne(ctx, bson.M{"username": username},
		bson.M{"$set": bson.M{"password_hash": hash, "session_nonce": nonce}})
	if err != nil {
		return "", err
	}
	return nonce, nil
}

// BackfillSessionNonces 为缺少会话标识的历史账号补一个，返回更新条数。
// 补齐之后，升级前签发的令牌（不带 sid）因为与库中记录不一致而全部失效。
func (s *Store) BackfillSessionNonces(ctx context.Context) (int64, error) {
	filter := bson.M{"$or": []bson.M{
		{"session_nonce": bson.M{"$exists": false}},
		{"session_nonce": ""},
		{"session_nonce": nil},
	}}
	cur, err := s.Users.Find(ctx, filter, options.Find().SetProjection(bson.M{"username": 1}))
	if err != nil {
		return 0, err
	}
	defer cur.Close(ctx)
	type row struct {
		Username string `bson:"username"`
	}
	var rows []row
	if err := cur.All(ctx, &rows); err != nil {
		return 0, err
	}
	var updated int64
	for _, r := range rows {
		if _, err := s.RotateSessionNonce(ctx, r.Username); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
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
		SessionNonce: newSessionNonce(),
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
