package scanner

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ParsePorts 解析 "80,443,1-1024" 形式的端口表达式。
func ParsePorts(spec string) ([]int, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("端口不能为空")
	}
	seen := map[int]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			from, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
			to, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err1 != nil || err2 != nil || from < 1 || to > 65535 || from > to {
				return nil, fmt.Errorf("端口范围不合法: %s", part)
			}
			for p := from; p <= to; p++ {
				seen[p] = true
			}
			continue
		}
		p, err := strconv.Atoi(part)
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("端口不合法: %s", part)
		}
		seen[p] = true
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("端口不能为空")
	}
	out := make([]int, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Ints(out)
	return out, nil
}

func PortsToSpec(ports []int) string {
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = strconv.Itoa(p)
	}
	return strings.Join(parts, ",")
}

func ChunkInts(items []int, size int) [][]int {
	if size <= 0 {
		size = 100
	}
	var out [][]int
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		out = append(out, items[i:end])
	}
	return out
}

func ChunkStrings(items []string, size int) [][]string {
	if size <= 0 {
		size = 32
	}
	var out [][]string
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		out = append(out, items[i:end])
	}
	return out
}
