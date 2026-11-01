package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

type Overview struct {
	TotalAssets  int64            `json:"totalAssets"`
	UniqueIPs    int64            `json:"uniqueIPs"`
	New24h       int64            `json:"new24h"`
	New7d        int64            `json:"new7d"`
	PrivateHosts int64            `json:"privateHosts"`
	PublicHosts  int64            `json:"publicHosts"`
	TopPorts     []Facet          `json:"topPorts"`
	TopServices  []Facet          `json:"topServices"`
	TopCountries []Facet          `json:"topCountries"`
	TopApps      []Facet          `json:"topApps"`
	Trend        []Trend          `json:"trend"`
	TaskCounts   map[string]int64 `json:"taskCounts"`
	GeneratedAt  time.Time        `json:"generatedAt"`
}

type Trend struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

func (s *Store) Count(ctx context.Context, filter bson.M) (int64, error) {
	return s.Hosts.CountDocuments(ctx, filter)
}

func (s *Store) UniqueIPCount(ctx context.Context) (int64, error) {
	cur, err := s.Hosts.Aggregate(ctx, []bson.M{
		{"$group": bson.M{"_id": "$ip"}},
		{"$count": "n"},
	})
	if err != nil {
		return 0, err
	}
	defer cur.Close(ctx)
	if cur.Next(ctx) {
		var row struct {
			N int64 `bson:"n"`
		}
		if err := cur.Decode(&row); err != nil {
			return 0, err
		}
		return row.N, nil
	}
	return 0, cur.Err()
}

// TrendDays 返回最近 days 天每日新增资产（按首次发现时间）。
func (s *Store) TrendDays(ctx context.Context, days int) ([]Trend, error) {
	since := time.Now().AddDate(0, 0, -days+1).Truncate(24 * time.Hour)
	cur, err := s.Hosts.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"first_seen": bson.M{"$gte": since}}},
		{"$group": bson.M{
			"_id":   bson.M{"$dateToString": bson.M{"format": "%Y-%m-%d", "date": "$first_seen"}},
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"_id": 1}},
	})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	byDay := map[string]int64{}
	for cur.Next(ctx) {
		var row struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, err
		}
		byDay[row.ID] = row.Count
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	out := make([]Trend, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		out = append(out, Trend{Date: d, Count: byDay[d]})
	}
	return out, nil
}

// BuildOverview 汇总首页统计所需的所有指标。
func (s *Store) BuildOverview(ctx context.Context) (*Overview, error) {
	ov := &Overview{GeneratedAt: time.Now()}
	var err error
	if ov.TotalAssets, err = s.Count(ctx, bson.M{}); err != nil {
		return nil, err
	}
	if ov.UniqueIPs, err = s.UniqueIPCount(ctx); err != nil {
		return nil, err
	}
	now := time.Now()
	ov.New24h, _ = s.Count(ctx, bson.M{"first_seen": bson.M{"$gte": now.Add(-24 * time.Hour)}})
	ov.New7d, _ = s.Count(ctx, bson.M{"first_seen": bson.M{"$gte": now.AddDate(0, 0, -7)}})
	ov.PrivateHosts, _ = s.Count(ctx, bson.M{"geo.private": true})
	ov.PublicHosts = ov.TotalAssets - ov.PrivateHosts
	ov.TopPorts, _ = s.topValues(ctx, "port", 10, false)
	ov.TopServices, _ = s.topValues(ctx, "service.name", 10, false)
	ov.TopCountries, _ = s.topValues(ctx, "geo.country_code", 10, false)
	ov.TopApps, _ = s.topValues(ctx, "apps", 14, true)
	ov.Trend, _ = s.TrendDays(ctx, 14)
	ov.TaskCounts, _ = s.CountTasksByStatus(ctx)
	return ov, nil
}

func (s *Store) topValues(ctx context.Context, field string, limit int, unwind bool) ([]Facet, error) {
	pipeline := []bson.M{}
	if unwind {
		pipeline = append(pipeline, bson.M{"$unwind": "$" + field})
	}
	pipeline = append(pipeline,
		bson.M{"$group": bson.M{"_id": bson.M{"$toString": "$" + field}, "count": bson.M{"$sum": 1}}},
		bson.M{"$match": bson.M{"_id": bson.M{"$nin": bson.A{"", "0", "null"}}}},
		bson.M{"$sort": bson.M{"count": -1}},
		bson.M{"$limit": limit},
	)
	cur, err := s.Hosts.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	out := []Facet{}
	for cur.Next(ctx) {
		var row struct {
			ID    string `bson:"_id"`
			Count int    `bson:"count"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, err
		}
		out = append(out, Facet{Value: row.ID, Count: row.Count})
	}
	return out, cur.Err()
}
