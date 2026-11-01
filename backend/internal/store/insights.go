package store

import (
	"context"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/expvintl/scopex/internal/model"
)

// 本文件负责「全球数据汇总」页所需的跨维度聚合。
//
// 口径说明：库里的原始取值（国家码、service.name、os 字符串、应用名）写法很杂，
// 直接做成饼图会出现上百个碎片。因此这里先按 model 包里的归类函数折叠成稳定键，
// 再按计数排序；前端只负责把键翻译成文案、把占比画成环形图。

// InsightSlice 是饼图/排行里的一个切片。
type InsightSlice struct {
	Key   string `json:"key"`             // 归一化后的键（大洲、品类、操作系统家族）；无归一化时等于 Label
	Label string `json:"label,omitempty"` // 原始取值（国家码、端口、应用名、ASN 组织）
	Count int64  `json:"count"`
}

// Insights 是汇总页的完整数据包。
type Insights struct {
	Total       int64          `json:"total"`
	UniqueIPs   int64          `json:"uniqueIPs"`
	Private     int64          `json:"private"`
	Public      int64          `json:"public"`
	AIAssets    int64          `json:"aiAssets"`
	CountryNum  int            `json:"countryNum"`
	ASNNum      int            `json:"asnNum"`
	Countries   []InsightSlice `json:"countries"`
	Continents  []InsightSlice `json:"continents"`
	Services    []InsightSlice `json:"services"`
	OSFamilies  []InsightSlice `json:"osFamilies"`
	AppCats     []InsightSlice `json:"appCategories"`
	AIApps      []InsightSlice `json:"aiApps"`
	Devices     []InsightSlice `json:"devices"`
	Ports       []InsightSlice `json:"ports"`
	Apps        []InsightSlice `json:"apps"`
	ASNs        []InsightSlice `json:"asns"`
	TLSIssuers  []InsightSlice `json:"tlsIssuers"`
	GeneratedAt time.Time      `json:"generatedAt"`
}

// bucketize 把原始取值折叠到统计键，并合并同类项后按计数倒序。
// label 保留原始取值，便于前端在提示里展示细节。
func bucketize(items []Facet, key func(string) string) []InsightSlice {
	sum := map[string]*InsightSlice{}
	for _, item := range items {
		k := key(item.Value)
		if k == "" {
			continue
		}
		if s, ok := sum[k]; ok {
			s.Count += int64(item.Count)
			continue
		}
		sum[k] = &InsightSlice{Key: k, Label: item.Value, Count: int64(item.Count)}
	}
	out := make([]InsightSlice, 0, len(sum))
	for _, s := range sum {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// plain 把 Facet 直接转成切片，用于端口、应用这类不需要归一化的维度。
func plain(items []Facet) []InsightSlice {
	out := make([]InsightSlice, 0, len(items))
	for _, item := range items {
		// 字段缺失时聚合会把空值也聚成一桶，这里直接丢掉，避免出现空标签的切片。
		if item.Value == "" {
			continue
		}
		out = append(out, InsightSlice{Key: item.Value, Label: item.Value, Count: int64(item.Count)})
	}
	return out
}

// BuildInsights 汇总全局分布数据。
// osFallbackLimit 限制兜底扫描的文档量：只为填充「系统分布」而全表扫描不划算，
// 先按 os 字段聚合，只有缺 os 的资产才需要读 CPE/banner。
const osFallbackLimit = 20000

// osFacets 汇总操作系统家族：主口径是 os 字段，缺失时用 CPE/banner 兜底。
// 返回的 Value 既可能是原始 os 字符串，也可能已经是家族键，交给 bucketize 折叠。
func (s *Store) osFacets(ctx context.Context) []Facet {
	oses, err := s.topValues(ctx, "os", 120, false)
	if err != nil {
		oses = []Facet{}
	}
	filter := bson.M{
		"os": "",
		"$or": []bson.M{
			{"cpe": bson.M{"$exists": true, "$ne": bson.A{}}},
			{"banner": bson.M{"$nin": bson.A{"", nil}}},
		},
	}
	opts := options.Find().
		SetProjection(bson.M{"os": 1, "cpe": 1, "banner": 1, "service.extra_info": 1}).
		SetLimit(osFallbackLimit)
	cur, err := s.Hosts.Find(ctx, filter, opts)
	if err != nil {
		return oses
	}
	defer cur.Close(ctx)
	seen := map[string]int{}
	for cur.Next(ctx) {
		var h model.Host
		if err := cur.Decode(&h); err != nil {
			continue
		}
		if fam := model.OSFamilyOf(&h); fam != "" {
			seen[fam]++
		}
	}
	for fam, count := range seen {
		oses = append(oses, Facet{Value: fam, Count: count})
	}
	return oses
}

func (s *Store) BuildInsights(ctx context.Context) (*Insights, error) {
	out := &Insights{GeneratedAt: time.Now()}
	total, err := s.Count(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	out.Total = total
	out.UniqueIPs, _ = s.UniqueIPCount(ctx)
	out.Private, _ = s.Count(ctx, bson.M{"geo.private": true})
	out.Public = out.Total - out.Private
	out.AIAssets, _ = s.Count(ctx, bson.M{"ai.kind": bson.M{"$exists": true}})

	countries, _ := s.topValues(ctx, "geo.country_code", 300, false)
	services, _ := s.topValues(ctx, "service.name", 120, false)
	oses := s.osFacets(ctx)
	devices, _ := s.topValues(ctx, "device", 30, false)
	ports, _ := s.topValues(ctx, "port", 20, false)
	apps, _ := s.topValues(ctx, "apps", 40, true)
	asns, _ := s.topValues(ctx, "asn.org", 20, false)
	issuers, _ := s.topValues(ctx, "tls.issuer", 20, false)

	out.Countries = plain(countries)
	out.ASNs = plain(asns)
	out.Devices = plain(devices)
	out.Ports = plain(ports)
	out.Apps = plain(apps)
	out.TLSIssuers = plain(issuers)
	out.Services = bucketize(services, model.ServiceCategory)
	out.OSFamilies = bucketize(oses, model.OSFamily)
	out.AppCats = bucketize(apps, model.AppCategory)
	aiApps := make([]Facet, 0, len(apps))
	for _, item := range apps {
		if model.AppCategory(item.Value) == "ai" {
			aiApps = append(aiApps, item)
		}
	}
	out.AIApps = plain(aiApps)
	// 大洲从国家码折算，未知国家码（内网地址、库缺失）单独归到 unknown。
	// 大洲的 Label 直接给稳定键（asia/unknown…），文案交给前端翻译；
	// 不能沿用原始国家码，否则接口消费方会看到 label="CN" 这种误导值。
	continentSum := map[string]int{}
	for _, c := range countries {
		if c.Value == "" {
			continue
		}
		key := model.Continent(c.Value)
		if key == "" {
			key = "unknown"
		}
		continentSum[key] += c.Count
	}
	out.Continents = make([]InsightSlice, 0, len(continentSum))
	for key, count := range continentSum {
		out.Continents = append(out.Continents, InsightSlice{Key: key, Label: key, Count: int64(count)})
	}
	sort.Slice(out.Continents, func(i, j int) bool {
		if out.Continents[i].Count != out.Continents[j].Count {
			return out.Continents[i].Count > out.Continents[j].Count
		}
		return out.Continents[i].Key < out.Continents[j].Key
	})

	countrySet := map[string]bool{}
	for _, c := range countries {
		if c.Value != "" {
			countrySet[c.Value] = true
		}
	}
	out.CountryNum = len(countrySet)
	asnSet := map[string]bool{}
	for _, a := range asns {
		if a.Value != "" {
			asnSet[a.Value] = true
		}
	}
	out.ASNNum = len(asnSet)
	return out, nil
}
