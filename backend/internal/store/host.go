package store

import (
	"context"
	"time"

	"github.com/expvintl/scopex/internal/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *Store) UpsertHost(ctx context.Context, h *model.Host) (bool, error) {
	set := bson.M{
		"ip":          h.IP,
		"ipnum":       h.IPNum,
		"port":        h.Port,
		"protocol":    h.Protocol,
		"service":     h.Service,
		"banner":      h.Banner,
		"title":       h.Title,
		"headers":     h.Headers,
		"header_text": h.HeaderText,
		"status_code": h.StatusCode,
		"apps":        h.Apps,
		"cpe":         h.CPE,
		"device":      h.Device,
		"geo":         h.Geo,
		"asn":         h.ASN,
		"scripts":     h.Scripts,
		"tags":        h.Tags,
		"source":      h.Source,
		"last_seen":   h.LastSeen,
	}
	if h.OS != "" {
		set["os"] = h.OS
	}
	if h.Favicon != nil {
		set["favicon"] = h.Favicon
	}
	if len(h.FaviconImg) > 0 {
		set["favicon_img"] = h.FaviconImg
	}
	if h.TLS != nil {
		set["tls"] = h.TLS
	}
	if h.AI != nil {
		set["ai"] = h.AI
	}
	update := bson.M{
		"$set":         set,
		"$setOnInsert": bson.M{"first_seen": h.FirstSeen},
		"$addToSet": bson.M{
			"hostnames": bson.M{"$each": h.Hostnames},
			"domains":   bson.M{"$each": h.Domains},
		},
	}
	// 重新探测时若已不再命中 AI 指纹，清掉旧结论，避免汇总页统计陈旧数据。
	if h.AI == nil {
		update["$unset"] = bson.M{"ai": ""}
	}
	opts := options.Update().SetUpsert(true)
	res, err := s.Hosts.UpdateOne(ctx, bson.M{"ip": h.IP, "port": h.Port, "protocol": h.Protocol}, update, opts)
	if err != nil {
		return false, err
	}
	return res.UpsertedCount > 0, nil
}

// ApplyNmap 用 nmap 的服务识别结果覆盖已有资产的 service/cpe/scripts。
func (s *Store) ApplyNmap(ctx context.Context, ip string, port int, proto string, svc model.Service, cpe []string, scripts map[string]string) error {
	set := bson.M{"service": svc}
	if len(cpe) > 0 {
		set["cpe"] = cpe
	}
	if len(scripts) > 0 {
		set["scripts"] = scripts
	}
	_, err := s.Hosts.UpdateOne(ctx, bson.M{"ip": ip, "port": port, "protocol": proto}, bson.M{"$set": set})
	return err
}

func (s *Store) GetFavicon(ctx context.Context, id any) ([]byte, error) {
	var doc struct {
		Img []byte `bson:"favicon_img"`
	}
	err := s.Hosts.FindOne(ctx, bson.M{"_id": id}, options.FindOne().SetProjection(bson.M{"favicon_img": 1})).Decode(&doc)
	return doc.Img, err
}

func (s *Store) GetHost(ctx context.Context, id any) (*model.Host, error) {
	var h model.Host
	if err := s.Hosts.FindOne(ctx, bson.M{"_id": id}).Decode(&h); err != nil {
		return nil, err
	}
	return &h, nil
}

type HostQuery struct {
	Filter  bson.M
	SortKey string
	Desc    bool
	Page    int
	Size    int
}

func (s *Store) SearchHosts(ctx context.Context, q HostQuery) ([]model.Host, int64, error) {
	total, err := s.Hosts.CountDocuments(ctx, q.Filter)
	if err != nil {
		return nil, 0, err
	}
	sortKey := q.SortKey
	switch sortKey {
	case "", "last_seen":
		sortKey = "last_seen"
	case "first_seen", "port", "ipnum", "status_code", "updated_at":
	default:
		sortKey = "last_seen"
	}
	dir := -1
	if !q.Desc {
		dir = 1
	}
	opts := options.Find().
		SetSort(bson.D{{Key: sortKey, Value: dir}}).
		SetSkip(int64((q.Page - 1) * q.Size)).
		SetLimit(int64(q.Size)).
		SetProjection(bson.M{"favicon_img": 0})
	cur, err := s.Hosts.Find(ctx, q.Filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var hosts []model.Host
	if err := cur.All(ctx, &hosts); err != nil {
		return nil, 0, err
	}
	return hosts, total, nil
}

type Facet struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type Facets struct {
	Ports     []Facet `json:"ports"`
	Services  []Facet `json:"services"`
	Countries []Facet `json:"countries"`
	Apps      []Facet `json:"apps"`
}

type facetRow struct {
	ID    any `bson:"_id"`
	Count int `bson:"count"`
}

func (s *Store) HostFacets(ctx context.Context, filter bson.M, limit int) (Facets, error) {
	group := func(field string) []bson.M {
		return []bson.M{
			{"$match": filter},
			{"$group": bson.M{"_id": field, "count": bson.M{"$sum": 1}}},
			{"$match": bson.M{"_id": bson.M{"$nin": bson.A{"", nil}}}},
			{"$sort": bson.M{"count": -1}},
			{"$limit": limit},
		}
	}
	pipeline := []bson.M{
		{"$facet": bson.M{
			"ports":     group("$port"),
			"services":  group("$service.name"),
			"countries": group("$geo.country_code"),
			"apps": []bson.M{
				{"$match": filter},
				{"$unwind": "$apps"},
				{"$group": bson.M{"_id": "$apps", "count": bson.M{"$sum": 1}}},
				{"$match": bson.M{"_id": bson.M{"$nin": bson.A{"", nil}}}},
				{"$sort": bson.M{"count": -1}},
				{"$limit": limit},
			},
		}},
	}
	cur, err := s.Hosts.Aggregate(ctx, pipeline)
	if err != nil {
		return Facets{}, err
	}
	defer cur.Close(ctx)
	var raw []struct {
		Ports     []facetRow `bson:"ports"`
		Services  []facetRow `bson:"services"`
		Countries []facetRow `bson:"countries"`
		Apps      []facetRow `bson:"apps"`
	}
	if err := cur.All(ctx, &raw); err != nil {
		return Facets{}, err
	}
	if len(raw) == 0 {
		return Facets{}, nil
	}
	convert := func(rows []facetRow) []Facet {
		out := make([]Facet, 0, len(rows))
		for _, r := range rows {
			if r.ID == nil {
				continue
			}
			out = append(out, Facet{Value: toStr(r.ID), Count: r.Count})
		}
		return out
	}
	return Facets{
		Ports:     convert(raw[0].Ports),
		Services:  convert(raw[0].Services),
		Countries: convert(raw[0].Countries),
		Apps:      convert(raw[0].Apps),
	}, nil
}

func (s *Store) DistinctHostValues(ctx context.Context, field string, limit int64) ([]string, error) {
	vals, err := s.Hosts.Distinct(ctx, field, bson.M{})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if str := toStr(v); str != "" {
			out = append(out, str)
		}
		if int64(len(out)) >= limit {
			break
		}
	}
	return out, nil
}

func (s *Store) CountHosts(ctx context.Context) (int64, error) {
	return s.Hosts.CountDocuments(ctx, bson.M{})
}

// DeleteHost 删除单条资产，不存在时返回 mongo.ErrNoDocuments。
func (s *Store) DeleteHost(ctx context.Context, id any) error {
	res, err := s.Hosts.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// DeleteHostsByIP 删除某个 IP 下的全部资产。
func (s *Store) DeleteHostsByIP(ctx context.Context, ip string) (int64, error) {
	res, err := s.Hosts.DeleteMany(ctx, bson.M{"ip": ip})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

// DeleteHostsByFilter 按条件批量删除资产，返回删除条数。
func (s *Store) DeleteHostsByFilter(ctx context.Context, filter bson.M) (int64, error) {
	res, err := s.Hosts.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

func (s *Store) DeleteHostsBefore(ctx context.Context, t time.Time) (int64, error) {
	res, err := s.Hosts.DeleteMany(ctx, bson.M{"last_seen": bson.M{"$lt": t}})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

func (s *Store) HostCollection() *mongo.Collection { return s.Hosts }

// HostsMissingGeo 返回缺少归属地信息的 IP（去重后最多 limit 个）。
func (s *Store) HostsMissingGeo(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 200
	}
	cur, err := s.Hosts.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"$or": bson.A{
			bson.M{"geo.country_code": ""},
			bson.M{"geo.country_code": bson.M{"$exists": false}},
		}}}},
		{{Key: "$group", Value: bson.M{"_id": "$ip"}}},
		{{Key: "$limit", Value: int64(limit)}},
	})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []struct {
		IP string `bson:"_id"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	ips := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.IP != "" {
			ips = append(ips, r.IP)
		}
	}
	return ips, nil
}

// SetHostGeo 覆写某个 IP 下全部资产的归属地与 ASN。
func (s *Store) SetHostGeo(ctx context.Context, ip string, g model.Geo, asn *model.ASN) (int64, error) {
	set := bson.M{"geo": g}
	if asn != nil {
		set["asn"] = asn
	}
	res, err := s.Hosts.UpdateMany(ctx, bson.M{"ip": ip}, bson.M{"$set": set})
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

func toStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case int32:
		return itoa(int64(t))
	case int64:
		return itoa(t)
	case int:
		return itoa(int64(t))
	case float64:
		return itoa(int64(t))
	case nil:
		return ""
	}
	return ""
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// PortBrief 是按 IP 聚合后展示的端口摘要。
type PortBrief struct {
	Port       int      `bson:"port" json:"port"`
	Protocol   string   `bson:"protocol" json:"protocol"`
	Service    string   `bson:"service" json:"service"`
	Product    string   `bson:"product" json:"product"`
	Title      string   `bson:"title" json:"title"`
	StatusCode int      `bson:"status_code" json:"statusCode"`
	Apps       []string `bson:"apps" json:"apps"`
}

// IPGroup 把同一 IP 的多个端口合并成一条资产记录。
type IPGroup struct {
	IP        string      `bson:"_id" json:"ip"`
	IPNum     uint32      `bson:"ipnum" json:"ipNum"`
	Geo       model.Geo   `bson:"geo" json:"geo"`
	Hostnames []string    `bson:"hostnames" json:"hostnames"`
	Domains   []string    `bson:"domains" json:"domains"`
	PortCount int         `bson:"port_count" json:"portCount"`
	Ports     []PortBrief `bson:"ports" json:"ports"`
	FirstSeen time.Time   `bson:"first_seen" json:"firstSeen"`
	LastSeen  time.Time   `bson:"last_seen" json:"lastSeen"`
}

type ipGroupDoc struct {
	Data  []IPGroup `bson:"data"`
	Total []struct {
		N int64 `bson:"n"`
	} `bson:"total"`
}

// SearchIPGroups 先按 IP 聚合再分页，Ports 只返回前 maxPorts 个端口。
func (s *Store) SearchIPGroups(ctx context.Context, q HostQuery, maxPorts int) ([]IPGroup, int64, error) {
	if maxPorts <= 0 {
		maxPorts = 12
	}
	sortKey := q.SortKey
	switch sortKey {
	case "first_seen", "port_count":
	case "ip":
		sortKey = "ipnum"
	case "", "last_seen":
		sortKey = "last_seen"
	default:
		sortKey = "last_seen"
	}
	dir := -1
	if !q.Desc {
		dir = 1
	}
	groupSort := map[string]string{
		"last_seen":  "last_seen",
		"first_seen": "first_seen",
		"port_count": "port_count",
		"ipnum":      "_id",
	}[sortKey]

	filter := q.Filter
	if filter == nil {
		filter = bson.M{}
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$sort", Value: bson.D{{Key: sortKey, Value: dir}}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$ip"},
			{Key: "ipnum", Value: bson.D{{Key: "$first", Value: "$ipnum"}}},
			{Key: "geo", Value: bson.D{{Key: "$first", Value: "$geo"}}},
			{Key: "hostnames", Value: bson.D{{Key: "$first", Value: "$hostnames"}}},
			{Key: "domains", Value: bson.D{{Key: "$first", Value: "$domains"}}},
			{Key: "first_seen", Value: bson.D{{Key: "$min", Value: "$first_seen"}}},
			{Key: "last_seen", Value: bson.D{{Key: "$max", Value: "$last_seen"}}},
			{Key: "port_count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "all_ports", Value: bson.D{{Key: "$push", Value: bson.M{
				"port":        "$port",
				"protocol":    "$protocol",
				"service":     "$service.name",
				"product":     "$service.product",
				"title":       "$title",
				"status_code": "$status_code",
				"apps":        "$apps",
			}}}},
		}}},
		{{Key: "$addFields", Value: bson.D{
			{Key: "ports", Value: bson.D{{Key: "$slice", Value: bson.A{"$all_ports", maxPorts}}}},
		}}},
		{{Key: "$project", Value: bson.M{"all_ports": 0}}},
		{{Key: "$facet", Value: bson.M{
			"data": bson.A{
				bson.M{"$sort": bson.D{{Key: groupSort, Value: dir}}},
				bson.M{"$skip": int64((q.Page - 1) * q.Size)},
				bson.M{"$limit": int64(q.Size)},
			},
			"total": bson.A{bson.M{"$count": "n"}},
		}}},
	}
	cur, err := s.Hosts.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var rows []ipGroupDoc
	if err := cur.All(ctx, &rows); err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return []IPGroup{}, 0, nil
	}
	total := int64(0)
	if len(rows[0].Total) > 0 {
		total = rows[0].Total[0].N
	}
	if rows[0].Data == nil {
		return []IPGroup{}, total, nil
	}
	return rows[0].Data, total, nil
}

// HostsByIP 返回某个 IP 下的全部端口资产。
func (s *Store) HostsByIP(ctx context.Context, ip string, limit int) ([]model.Host, error) {
	if limit <= 0 {
		limit = 500
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "port", Value: 1}}).
		SetLimit(int64(limit)).
		SetProjection(bson.M{"favicon_img": 0})
	cur, err := s.Hosts.Find(ctx, bson.M{"ip": ip}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var hosts []model.Host
	if err := cur.All(ctx, &hosts); err != nil {
		return nil, err
	}
	return hosts, nil
}
