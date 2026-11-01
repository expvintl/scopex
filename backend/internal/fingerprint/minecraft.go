package fingerprint

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/expvintl/scopex/internal/model"
	"github.com/expvintl/scopex/internal/util"
)

const minecraftProtocol = 767

var versionPattern = regexp.MustCompile(`^\d+\.\d+(\.\d+)?$`)

type minecraftStatus struct {
	Version struct {
		Name     string `json:"name"`
		Protocol int    `json:"protocol"`
	} `json:"version"`
	Players struct {
		Max    int `json:"max"`
		Online int `json:"online"`
	} `json:"players"`
	Description json.RawMessage `json:"description"`
	Favicon     string          `json:"favicon"`
	ModInfo     struct {
		Type string `json:"type"`
	} `json:"modinfo"`
}

// ProbeMinecraft 用服务器列表 ping 协议识别 Minecraft Java 版服务。
func (p *Prober) probeMinecraft(ctx context.Context, ip string, port int) *Result {
	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: p.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	} else {
		_ = conn.SetDeadline(time.Now().Add(p.Timeout))
	}

	if _, err := conn.Write(handshakePacket(ip, port)); err != nil {
		return nil
	}
	if _, err := conn.Write([]byte{0x01, 0x00}); err != nil {
		return nil
	}

	br := bufio.NewReaderSize(conn, 64*1024)
	length, err := readVarInt(br)
	if err != nil || length <= 0 || length > 2<<20 {
		return nil
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(br, payload); err != nil {
		return nil
	}
	rd := bytes.NewReader(payload)
	if _, err := readVarInt(rd); err != nil {
		return nil
	}
	raw, err := readMCString(rd)
	if err != nil {
		return nil
	}

	var st minecraftStatus
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return nil
	}
	return p.minecraftResult(&st, raw)
}

func (p *Prober) minecraftResult(st *minecraftStatus, raw string) *Result {
	motd := stripMinecraftFormatting(parseMOTD(st.Description))
	service := model.Service{
		Name:       "minecraft",
		Product:    "Java Edition",
		Version:    st.Version.Name,
		ExtraInfo:  fmt.Sprintf("玩家 %d/%d", st.Players.Online, st.Players.Max),
		Method:     "minecraft",
		Confidence: 98,
	}
	if st.Version.Protocol > 0 {
		service.ExtraInfo += fmt.Sprintf(" · 协议 %d", st.Version.Protocol)
	}
	if motd == "" {
		motd = "Minecraft Server"
	}
	banner := fmt.Sprintf("Minecraft %s · 协议 %d · 玩家 %d/%d",
		st.Version.Name, st.Version.Protocol, st.Players.Online, st.Players.Max)
	res := &Result{
		Service: service,
		Title:   motd,
		Banner:  util.Truncate(banner, 300),
		Apps:    []string{"minecraft"},
		CPE:     []string{"cpe:/a:mojang:minecraft"},
	}
	if versionPattern.MatchString(st.Version.Name) {
		res.CPE = []string{"cpe:/a:mojang:minecraft:" + st.Version.Name}
	}
	if st.ModInfo.Type != "" && st.ModInfo.Type != "FML" {
		res.Apps = append(res.Apps, strings.ToLower(st.ModInfo.Type))
	}
	if st.Players.Max > 0 {
		res.Apps = append(res.Apps, "minecraft")
	}
	if data := decodeMinecraftFavicon(st.Favicon); len(data) > 0 {
		hash := FaviconHash(data)
		res.Favicon = &model.Favicon{MMH3: hash, Size: len(data)}
		res.FaviconImg = data
		res.Favicon.MD5 = md5Hex(data)
	}
	return res
}

func handshakePacket(host string, port int) []byte {
	var body bytes.Buffer
	writeVarInt(&body, 0x00)
	writeVarInt(&body, minecraftProtocol)
	writeMCString(&body, host)
	body.WriteByte(byte(port >> 8))
	body.WriteByte(byte(port))
	writeVarInt(&body, 1)

	var out bytes.Buffer
	writeVarInt(&out, int32(body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}

func writeVarInt(w io.ByteWriter, v int32) {
	u := uint32(v)
	for {
		if u&^0x7F == 0 {
			_ = w.WriteByte(byte(u))
			return
		}
		_ = w.WriteByte(byte(u&0x7F | 0x80))
		u >>= 7
	}
}

func readVarInt(r io.ByteReader) (int32, error) {
	var value uint32
	for i := 0; i < 5; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		value |= uint32(b&0x7F) << (7 * uint(i))
		if b&0x80 == 0 {
			return int32(value), nil
		}
	}
	return 0, fmt.Errorf("varint 过长")
}

type byteStringWriter interface {
	io.ByteWriter
	io.StringWriter
}

func writeMCString(w byteStringWriter, s string) {
	writeVarInt(w, int32(len(s)))
	w.WriteString(s)
}

func readMCString(r *bytes.Reader) (string, error) {
	n, err := readVarInt(r)
	if err != nil {
		return "", err
	}
	if n < 0 || int(n) > r.Len() {
		return "", fmt.Errorf("字符串长度非法")
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func parseMOTD(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return plain
	}
	var rich struct {
		Text  string            `json:"text"`
		Extra []json.RawMessage `json:"extra"`
	}
	if err := json.Unmarshal(raw, &rich); err != nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(rich.Text)
	for _, part := range rich.Extra {
		sb.WriteString(parseMOTD(part))
	}
	return sb.String()
}

func stripMinecraftFormatting(s string) string {
	var sb strings.Builder
	skip := false
	for _, r := range s {
		if skip {
			skip = false
			continue
		}
		if r == '§' {
			skip = true
			continue
		}
		sb.WriteRune(r)
	}
	return strings.TrimSpace(sb.String())
}

func md5Hex(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

func decodeMinecraftFavicon(s string) []byte {
	if s == "" {
		return nil
	}
	if i := strings.Index(s, "base64,"); i >= 0 {
		s = s[i+len("base64,"):]
	}
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(data) == 0 {
		return nil
	}
	return data
}
