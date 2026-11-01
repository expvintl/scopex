package fingerprint

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// startFakeMinecraft 启动一个只响应状态查询的 Minecraft 服务端。
func startFakeMinecraft(t *testing.T, status string) (string, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(3 * time.Second))
				buf := make([]byte, 1024)
				if _, err := c.Read(buf); err != nil {
					return
				}
				var body strings.Builder
				writeVarInt(&body, 0)
				writeMCString(&body, status)

				var out strings.Builder
				writeVarInt(&out, int32(body.Len()))
				out.WriteString(body.String())
				_, _ = c.Write([]byte(out.String()))
			}(conn)
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

func TestProbeMinecraft(t *testing.T) {
	status := `{"version":{"name":"1.20.4","protocol":765},"players":{"max":20,"online":5},` +
		`"description":{"text":"§aHello ","extra":[{"text":"World"}]},` +
		`"favicon":"data:image/png;base64,iVBORw0KGgo="}`
	ip, port := startFakeMinecraft(t, status)

	p := NewProber(3 * time.Second)
	res := p.probeMinecraft(context.Background(), ip, port)
	if res == nil {
		t.Fatal("未识别为 Minecraft 服务")
	}
	if res.Service.Name != "minecraft" || res.Service.Version != "1.20.4" {
		t.Fatalf("服务识别异常: %+v", res.Service)
	}
	if res.Title != "Hello World" {
		t.Fatalf("MOTD 解析异常: %q", res.Title)
	}
	if !strings.Contains(res.Service.ExtraInfo, "5/20") {
		t.Fatalf("玩家数异常: %q", res.Service.ExtraInfo)
	}
	if res.Favicon == nil || res.FaviconImg == nil {
		t.Fatal("未解析服务器图标")
	}
	if len(res.CPE) == 0 || res.CPE[0] != "cpe:/a:mojang:minecraft:1.20.4" {
		t.Fatalf("CPE 异常: %v", res.CPE)
	}
}

func TestProbeMinecraftPlainMOTD(t *testing.T) {
	status := `{"version":{"name":"1.8.9","protocol":47},"players":{"max":0,"online":0},"description":"A Minecraft Server"}`
	ip, port := startFakeMinecraft(t, status)

	p := NewProber(3 * time.Second)
	res := p.probeMinecraft(context.Background(), ip, port)
	if res == nil || res.Title != "A Minecraft Server" {
		t.Fatalf("纯文本 MOTD 解析异常: %#v", res)
	}
	if res.Favicon != nil {
		t.Fatal("无图标时不应产生 favicon")
	}
}
