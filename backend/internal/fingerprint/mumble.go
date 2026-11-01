package fingerprint

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/expvintl/scopex/internal/model"
)

// probeMumble 读取 Mumble 服务端主动下发的 Version 数据包。
func (p *Prober) probeMumble(ctx context.Context, ip string, port int) *Result {
	conn, err := p.dialProbe(ctx, ip, port)
	if err != nil {
		return nil
	}
	defer conn.Close()

	header := readAtMost(conn, 12)
	if len(header) < 12 {
		return nil
	}
	version := binary.BigEndian.Uint32(header[0:4])
	length := int(binary.BigEndian.Uint32(header[8:12]))
	if length <= 0 || length > 1<<16 {
		return nil
	}
	payload := readAtMost(conn, length)

	major := int(version>>16) & 0xffff
	minor := int(version>>8) & 0xff
	patch := int(version) & 0xff
	fields := protoFields(payload)

	release := strings.TrimSpace(fields[2])
	if release == "" {
		release = fmt.Sprintf("%d.%d.%d", major, minor, patch)
	}

	var parts []string
	parts = append(parts, "Mumble "+release)
	osName := strings.TrimSpace(fields[3])
	osVersion := strings.TrimSpace(fields[4])
	if osName != "" {
		value := osName
		if osVersion != "" {
			value += " " + osVersion
		}
		parts = append(parts, value)
	}

	banner := strings.Join(parts, " · ")
	res := &Result{
		Service: model.Service{
			Name:       "mumble",
			Product:    "Mumble",
			Version:    release,
			ExtraInfo:  strings.TrimSpace(osName + " " + osVersion),
			Method:     "mumble",
			Confidence: 95,
			Banner:     banner,
		},
		Banner:  banner,
		Apps:    []string{"mumble"},
		CPE:     []string{"cpe:/a:mumble:mumble"},
		Headers: map[string]string{},
	}
	return res
}

// protoFields 解析 protobuf 消息里所有长度分隔字段，返回字段号到文本的映射。
func protoFields(data []byte) map[int]string {
	fields := map[int]string{}
	for len(data) > 0 {
		key, rest, ok := readUvarint(data)
		if !ok {
			break
		}
		data = rest
		field := int(key >> 3)
		switch key & 7 {
		case 0:
			_, next, ok := readUvarint(data)
			if !ok {
				return fields
			}
			data = next
		case 2:
			size, next, ok := readUvarint(data)
			if !ok || size > uint64(len(next)) {
				return fields
			}
			value := next[:size]
			data = next[size:]
			if utf8.Valid(value) && printable(string(value)) {
				fields[field] = string(value)
			}
		default:
			return fields
		}
	}
	return fields
}

func readUvarint(data []byte) (uint64, []byte, bool) {
	var value uint64
	for shift := uint(0); shift < 64 && len(data) > 0; shift += 7 {
		current := data[0]
		data = data[1:]
		value |= uint64(current&0x7f) << shift
		if current < 0x80 {
			return value, data, true
		}
	}
	return 0, nil, false
}

func printable(value string) bool {
	for _, r := range value {
		if r < 0x20 && r != '\t' {
			return false
		}
	}
	return true
}
