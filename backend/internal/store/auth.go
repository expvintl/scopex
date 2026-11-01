package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/expvintl/scopex/internal/util"
)

const authSettingsID = "auth"

// LoadOrCreateAuthSecret 返回 JWT 签名密钥。
//
// 通过环境变量指定时直接用；未指定时在 settings 集合里生成并保存一个随机密钥。
// 密钥落库带来两个必要的性质：
//   - 不再存在“仓库里公开的默认密钥”，无法离线伪造管理员令牌；
//   - 数据库被整体清空后密钥随之更换，此前签发的所有令牌立刻失效。
func (s *Store) LoadOrCreateAuthSecret(ctx context.Context, fromEnv string) (string, bool, error) {
	if fromEnv != "" {
		return fromEnv, false, nil
	}
	res, err := s.Settings.UpdateOne(ctx,
		bson.M{"_id": authSettingsID},
		bson.M{"$setOnInsert": bson.M{
			"jwt_secret": util.RandHex(32),
			"created_at": time.Now(),
		}},
		options.Update().SetUpsert(true),
	)
	// 多实例同时启动时，后到的那个会撞上 _id 唯一键，此时直接读已有的密钥即可。
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		return "", false, fmt.Errorf("生成签名密钥失败: %w", err)
	}
	created := err == nil && res != nil && res.UpsertedID != nil

	var doc struct {
		JWTSecret string `bson:"jwt_secret"`
	}
	if err := s.Settings.FindOne(ctx, bson.M{"_id": authSettingsID}).Decode(&doc); err != nil {
		return "", false, fmt.Errorf("读取签名密钥失败: %w", err)
	}
	if len(doc.JWTSecret) < 32 {
		return "", false, errors.New("数据库中的签名密钥无效")
	}
	return doc.JWTSecret, created, nil
}
