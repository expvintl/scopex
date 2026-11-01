package util

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
	"unicode/utf8"
)

// IPRange 是一个闭区间 IPv4 段。
type IPRange struct {
	Start uint32
	End   uint32
}

func (r IPRange) Count() uint64 { return uint64(r.End-r.Start) + 1 }

func (r IPRange) String() string {
	if r.Start == r.End {
		return Uint32ToIP(r.Start).String()
	}
	return fmt.Sprintf("%s-%s", Uint32ToIP(r.Start), Uint32ToIP(r.End))
}

func IPToUint32(ip net.IP) uint32 {
	v4 := ip.To4()
	if v4 == nil {
		return 0
	}
	return uint32(v4[0])<<24 | uint32(v4[1])<<16 | uint32(v4[2])<<8 | uint32(v4[3])
}

func Uint32ToIP(n uint32) net.IP {
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}

// ParseTarget 接受单 IP、CIDR 或 "a.b.c.d-e.f.g.h" 形式的短横线区间。
func ParseTarget(s string) (IPRange, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return IPRange{}, errors.New("empty target")
	}
	if strings.Contains(s, "/") {
		_, netw, err := net.ParseCIDR(s)
		if err != nil {
			return IPRange{}, fmt.Errorf("invalid cidr %q: %w", s, err)
		}
		if netw.IP.To4() == nil {
			return IPRange{}, fmt.Errorf("%q: only IPv4 is supported", s)
		}
		start := IPToUint32(netw.IP)
		ones, _ := netw.Mask.Size()
		size := uint32(1) << uint(32-ones)
		return IPRange{Start: start, End: start + size - 1}, nil
	}
	if i := strings.Index(s, "-"); i > 0 {
		from, to := strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
		a, b := net.ParseIP(from), net.ParseIP(to)
		if a == nil || b == nil || a.To4() == nil || b.To4() == nil {
			return IPRange{}, fmt.Errorf("invalid range %q", s)
		}
		start, end := IPToUint32(a), IPToUint32(b)
		if end < start {
			return IPRange{}, fmt.Errorf("invalid range %q: end before start", s)
		}
		return IPRange{Start: start, End: end}, nil
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return IPRange{}, fmt.Errorf("invalid ip %q", s)
	}
	if ip.To4() == nil {
		return IPRange{}, fmt.Errorf("%q: only IPv4 is supported", s)
	}
	n := IPToUint32(ip)
	return IPRange{Start: n, End: n}, nil
}

// ParseTargets 解析并合并多目标，返回去重后的区间与总 IP 数。
func ParseTargets(list []string) ([]IPRange, uint64, error) {
	var ranges []IPRange
	var total uint64
	for _, raw := range list {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '\n' || r == ' ' || r == '\t' }) {
			r, err := ParseTarget(part)
			if err != nil {
				return nil, 0, err
			}
			ranges = append(ranges, r)
			total += r.Count()
		}
	}
	if len(ranges) == 0 {
		return nil, 0, errors.New("no valid target")
	}
	return ranges, total, nil
}

func IsPrivateIP(ip net.IP) bool {
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	for _, c := range []struct{ start, end uint32 }{
		{0x0A000000, 0x0AFFFFFF}, // 10.0.0.0/8
		{0xAC100000, 0xAC1FFFFF}, // 172.16.0.0/12
		{0xC0A80000, 0xC0A8FFFF}, // 192.168.0.0/16
		{0x7F000000, 0x7FFFFFFF}, // loopback
		{0xA9FE0000, 0xA9FEFFFF}, // link-local
		{0x64400000, 0x647FFFFF}, // CGNAT 100.64/10
	} {
		n := IPToUint32(v4)
		if n >= c.start && n <= c.end {
			return true
		}
	}
	return false
}

// Truncate 按 rune 截断，避免把 UTF-8 切坏。
func Truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// RandHex 生成 n 字节随机数的十六进制串。
func RandHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("0", n*2)
	}
	return hex.EncodeToString(b)
}

// RandPassword 生成易读的随机密码（去掉易混淆字符）。
func RandPassword(n int) string {
	const alphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			b[i] = alphabet[i%len(alphabet)]
			continue
		}
		b[i] = alphabet[idx.Int64()]
	}
	return string(b)
}
