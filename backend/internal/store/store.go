package store

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Store struct {
	Client   *mongo.Client
	DB       *mongo.Database
	Hosts    *mongo.Collection
	Tasks    *mongo.Collection
	TaskLogs *mongo.Collection
	Users    *mongo.Collection
	Settings *mongo.Collection
}

func Open(ctx context.Context, uri, dbName string) (*Store, error) {
	cli, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetConnectTimeout(8*time.Second))
	if err != nil {
		return nil, err
	}
	if err := cli.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}
	db := cli.Database(dbName)
	s := &Store{
		Client:   cli,
		DB:       db,
		Hosts:    db.Collection("hosts"),
		Tasks:    db.Collection("tasks"),
		TaskLogs: db.Collection("task_logs"),
		Users:    db.Collection("users"),
		Settings: db.Collection("settings"),
	}
	if err := s.ensureIndexes(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) ensureIndexes(ctx context.Context) error {
	_, err := s.Hosts.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "ip", Value: 1}, {Key: "port", Value: 1}, {Key: "protocol", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_asset"),
		},
		{Keys: bson.D{{Key: "ipnum", Value: 1}}, Options: options.Index().SetName("ipnum")},
		{Keys: bson.D{{Key: "port", Value: 1}}, Options: options.Index().SetName("port")},
		{Keys: bson.D{{Key: "service.name", Value: 1}}, Options: options.Index().SetName("service")},
		{Keys: bson.D{{Key: "apps", Value: 1}}, Options: options.Index().SetName("apps")},
		{Keys: bson.D{{Key: "geo.country_code", Value: 1}}, Options: options.Index().SetName("country")},
		{Keys: bson.D{{Key: "geo.private", Value: 1}}, Options: options.Index().SetName("private")},
		{Keys: bson.D{{Key: "favicon.mmh3", Value: 1}}, Options: options.Index().SetName("favicon_mmh3")},
		{Keys: bson.D{{Key: "last_seen", Value: -1}}, Options: options.Index().SetName("last_seen")},
		{Keys: bson.D{{Key: "ip", Value: 1}, {Key: "last_seen", Value: -1}}, Options: options.Index().SetName("ip_seen")},
	})
	if err != nil {
		return fmt.Errorf("create host indexes: %w", err)
	}
	_, err = s.Tasks.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "created_at", Value: -1}}, Options: options.Index().SetName("created_at")},
		{Keys: bson.D{{Key: "status", Value: 1}}, Options: options.Index().SetName("status")},
	})
	if err != nil {
		return fmt.Errorf("create task indexes: %w", err)
	}
	_, err = s.TaskLogs.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "task_id", Value: 1}, {Key: "seq", Value: 1}}, Options: options.Index().SetUnique(true).SetName("task_seq")},
	})
	if err != nil {
		return fmt.Errorf("create task log indexes: %w", err)
	}
	_, err = s.Users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "username", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("uniq_username"),
	})
	if err != nil {
		return fmt.Errorf("create user index: %w", err)
	}
	return nil
}

func (s *Store) Close(ctx context.Context) error { return s.Client.Disconnect(ctx) }
