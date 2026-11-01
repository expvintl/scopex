package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"github.com/expvintl/scopex/internal/model"
)

func (s *Store) CreateTask(ctx context.Context, t *model.Task) error {
	res, err := s.Tasks.InsertOne(ctx, t)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		t.ID = oid
	}
	return nil
}

func (s *Store) GetTask(ctx context.Context, id primitive.ObjectID) (*model.Task, error) {
	var t model.Task
	if err := s.Tasks.FindOne(ctx, bson.M{"_id": id}).Decode(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) ListTasks(ctx context.Context, status string, page, size int) ([]model.Task, int64, error) {
	filter := bson.M{}
	if status != "" && status != "all" {
		filter["status"] = status
	}
	total, err := s.Tasks.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64((page - 1) * size)).
		SetLimit(int64(size))
	cur, err := s.Tasks.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var tasks []model.Task
	if err := cur.All(ctx, &tasks); err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

func (s *Store) UpdateTask(ctx context.Context, id primitive.ObjectID, set bson.M) error {
	if len(set) == 0 {
		return nil
	}
	_, err := s.Tasks.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	return err
}

func (s *Store) IncTaskStat(ctx context.Context, id primitive.ObjectID, field string, n int64) error {
	_, err := s.Tasks.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$inc": bson.M{field: n, "log_seq": 0},
	})
	return err
}

func (s *Store) DeleteTask(ctx context.Context, id primitive.ObjectID) error {
	if _, err := s.TaskLogs.DeleteMany(ctx, bson.M{"task_id": id}); err != nil {
		return err
	}
	_, err := s.Tasks.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// DeleteFinishedTasks 删除指定状态的已结束任务及其日志。
func (s *Store) DeleteFinishedTasks(ctx context.Context, statuses []string) (int64, error) {
	filter := bson.M{"status": bson.M{"$in": statuses}}
	cur, err := s.Tasks.Find(ctx, filter, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return 0, err
	}
	defer cur.Close(ctx)

	var rows []struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	ids := make([]primitive.ObjectID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	if _, err := s.TaskLogs.DeleteMany(ctx, bson.M{"task_id": bson.M{"$in": ids}}); err != nil {
		return 0, err
	}
	res, err := s.Tasks.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

// RecoverTasks 把所有未结束的任务标记为 interrupted（服务重启后调用）。
func (s *Store) RecoverTasks(ctx context.Context) (int64, error) {
	now := time.Now()
	res, err := s.Tasks.UpdateMany(ctx,
		bson.M{"status": bson.M{"$in": bson.A{model.TaskPending, model.TaskRunning}}},
		bson.M{"$set": bson.M{"status": model.TaskInterrupted, "error": "server restarted", "updated_at": now, "finished_at": now}},
	)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

func (s *Store) AppendLogs(ctx context.Context, taskID primitive.ObjectID, entries []model.TaskLog) error {
	if len(entries) == 0 {
		return nil
	}
	docs := make([]any, len(entries))
	for i, e := range entries {
		e.TaskID = taskID
		docs[i] = e
	}
	_, err := s.TaskLogs.InsertMany(ctx, docs, options.InsertMany().SetOrdered(false))
	return err
}

func (s *Store) TaskLogsAfter(ctx context.Context, taskID primitive.ObjectID, after int64, limit int64) ([]model.TaskLog, error) {
	cur, err := s.TaskLogs.Find(ctx,
		bson.M{"task_id": taskID, "seq": bson.M{"$gt": after}},
		options.Find().SetSort(bson.D{{Key: "seq", Value: 1}}).SetLimit(limit),
	)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var logs []model.TaskLog
	if err := cur.All(ctx, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}

func (s *Store) NextLogSeq(ctx context.Context, taskID primitive.ObjectID, n int64) (int64, error) {
	var res struct {
		LogSeq int64 `bson:"log_seq"`
	}
	err := s.Tasks.FindOneAndUpdate(ctx,
		bson.M{"_id": taskID},
		bson.M{"$inc": bson.M{"log_seq": n}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&res)
	if err != nil {
		return 0, err
	}
	return res.LogSeq - n + 1, nil
}

// AggregateTaskStats 返回各状态任务数量。
func (s *Store) CountTasksByStatus(ctx context.Context) (map[string]int64, error) {
	cur, err := s.Tasks.Aggregate(ctx, []bson.M{
		{"$group": bson.M{"_id": "$status", "count": bson.M{"$sum": 1}}},
	})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	out := map[string]int64{}
	for cur.Next(ctx) {
		var row struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, err
		}
		out[row.ID] = row.Count
	}
	return out, cur.Err()
}

func IsNotFound(err error) bool { return err == mongo.ErrNoDocuments }
