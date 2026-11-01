package store

import "testing"

func TestBucketizeMergesCategories(t *testing.T) {
	items := []Facet{
		{Value: "nginx", Count: 5},
		{Value: "http", Count: 9},
		{Value: "Ollama", Count: 3},
		{Value: "ComfyUI", Count: 2},
		{Value: "", Count: 7},
	}
	got := bucketize(items, func(v string) string {
		if v == "" {
			return ""
		}
		switch v {
		case "Ollama", "ComfyUI":
			return "ai"
		}
		return "web"
	})
	if len(got) != 2 {
		t.Fatalf("应合并成 2 个切片，实际 %d: %+v", len(got), got)
	}
	if got[0].Key != "web" || got[0].Count != 14 {
		t.Fatalf("web 切片不符: %+v", got[0])
	}
	if got[1].Key != "ai" || got[1].Count != 5 {
		t.Fatalf("ai 切片不符: %+v", got[1])
	}
}

func TestBucketizeUnknownContinent(t *testing.T) {
	got := bucketize([]Facet{{Value: "CN", Count: 2}, {Value: "ZZ", Count: 1}}, func(v string) string {
		if v == "CN" {
			return "asia"
		}
		return "unknown"
	})
	if len(got) != 2 || got[0].Key != "asia" || got[1].Key != "unknown" {
		t.Fatalf("未知国家码应单独归到 unknown: %+v", got)
	}
}

func TestPlainKeepsOrder(t *testing.T) {
	got := plain([]Facet{{Value: "443", Count: 30}, {Value: "80", Count: 12}})
	if len(got) != 2 || got[0].Key != "443" || got[0].Label != "443" || got[0].Count != 30 {
		t.Fatalf("plain 结果不符: %+v", got)
	}
}
