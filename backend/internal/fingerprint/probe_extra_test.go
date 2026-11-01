package fingerprint

import (
	"context"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func startFakeServer(t *testing.T, handler func(net.Conn)) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				handler(conn)
			}()
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

func encodeUTF16(value string) []byte {
	units := utf16.Encode([]rune(value))
	out := make([]byte, 0, len(units)*2)
	for _, unit := range units {
		out = binary.LittleEndian.AppendUint16(out, unit)
	}
	return out
}

func ntlmType2Fixture(host, domain, dnsHost, dnsDomain string) []byte {
	var av []byte
	add := func(id uint16, value string) {
		raw := encodeUTF16(value)
		av = binary.LittleEndian.AppendUint16(av, id)
		av = binary.LittleEndian.AppendUint16(av, uint16(len(raw)))
		av = append(av, raw...)
	}
	add(1, host)
	add(2, domain)
	add(3, dnsHost)
	add(4, dnsDomain)
	av = binary.LittleEndian.AppendUint16(av, 0)
	av = binary.LittleEndian.AppendUint16(av, 0)

	blob := make([]byte, 56)
	copy(blob, []byte("NTLMSSP\x00"))
	binary.LittleEndian.PutUint32(blob[8:], 2)
	binary.LittleEndian.PutUint32(blob[20:], 0xE28A8235)
	blob[48] = 10
	blob[49] = 0
	binary.LittleEndian.PutUint16(blob[50:], 19045)
	binary.LittleEndian.PutUint16(blob[40:], uint16(len(av)))
	binary.LittleEndian.PutUint32(blob[44:], 56)
	return append(blob, av...)
}

func smb2Response(command uint16, body []byte) []byte {
	header := make([]byte, 64)
	copy(header, []byte{0xfe, 'S', 'M', 'B'})
	binary.LittleEndian.PutUint16(header[4:], 64)
	binary.LittleEndian.PutUint16(header[12:], command)
	binary.LittleEndian.PutUint64(header[40:], 0x1122334455667788)
	return append(header, body...)
}

func smbFakeHandler(token []byte) func(net.Conn) {
	return func(conn net.Conn) {
		_, _ = conn.Read(make([]byte, 4096))

		body := make([]byte, 64)
		binary.LittleEndian.PutUint16(body[0:], 65)
		binary.LittleEndian.PutUint16(body[4:], 0x0311)
		_, _ = conn.Write(smb2Response(0x0000, body))

		_, _ = conn.Read(make([]byte, 4096))

		setup := make([]byte, 8)
		binary.LittleEndian.PutUint16(setup[0:], 9)
		binary.LittleEndian.PutUint16(setup[4:], 72)
		binary.LittleEndian.PutUint16(setup[6:], uint16(len(token)))
		_, _ = conn.Write(smb2Response(0x0001, append(setup, token...)))
	}
}

func TestProbeDispatchByHint(t *testing.T) {
	token := ntlmType2Fixture("WIN-SRV", "CORP", "win-srv.corp.local", "corp.local")
	smbPort := startFakeServer(t, smbFakeHandler(token))
	prober := NewProber(2 * time.Second)

	res := prober.Probe(context.Background(), "127.0.0.1", smbPort, "microsoft-ds")
	if res == nil || res.Service.Name != "smb" || res.Service.Version != "3.1.1" {
		t.Fatalf("smb dispatch failed: %+v", res)
	}

	mumblePort := startFakeServer(t, func(conn net.Conn) {
		payload := []byte{0x12, 0x07}
		payload = append(payload, []byte("1.4.287")...)
		header := make([]byte, 12)
		binary.BigEndian.PutUint32(header[8:], uint32(len(payload)))
		_, _ = conn.Write(append(header, payload...))
	})
	res = prober.Probe(context.Background(), "127.0.0.1", mumblePort, "mumble")
	if res == nil || res.Service.Name != "mumble" || res.Service.Version != "1.4.287" {
		t.Fatalf("mumble dispatch failed: %+v", res)
	}
}

func TestProbeSMBWindows(t *testing.T) {
	token := ntlmType2Fixture("WIN-SRV", "CORP", "win-srv.corp.local", "corp.local")
	port := startFakeServer(t, func(conn net.Conn) {
		_, _ = conn.Read(make([]byte, 4096))

		body := make([]byte, 64)
		binary.LittleEndian.PutUint16(body[0:], 65)
		binary.LittleEndian.PutUint16(body[4:], 0x0311)
		_, _ = conn.Write(smb2Response(0x0000, body))

		_, _ = conn.Read(make([]byte, 4096))

		setup := make([]byte, 8)
		binary.LittleEndian.PutUint16(setup[0:], 9)
		binary.LittleEndian.PutUint16(setup[4:], 72)
		binary.LittleEndian.PutUint16(setup[6:], uint16(len(token)))
		_, _ = conn.Write(smb2Response(0x0001, append(setup, token...)))
	})

	prober := NewProber(2 * time.Second)
	res := prober.probeSMB(context.Background(), "127.0.0.1", port)
	if res == nil {
		t.Fatal("expected smb result")
	}
	if res.Service.Name != "smb" {
		t.Fatalf("service = %q", res.Service.Name)
	}
	if res.Service.Version != "3.1.1" {
		t.Fatalf("version = %q", res.Service.Version)
	}
	if !strings.Contains(res.Banner, "WIN-SRV") || !strings.Contains(res.Banner, "CORP") {
		t.Fatalf("banner = %q", res.Banner)
	}
	if res.Device == "" {
		t.Fatal("expected os device name")
	}
	if len(res.Hostnames) == 0 || res.Hostnames[0] != "WIN-SRV" {
		t.Fatalf("hostnames = %v", res.Hostnames)
	}
	if !strings.HasPrefix(res.OS, "Windows 10") {
		t.Fatalf("os = %q", res.OS)
	}
	if len(res.Domains) == 0 || res.Domains[0] != "CORP" {
		t.Fatalf("domains = %v", res.Domains)
	}
}

func TestProbeMumble(t *testing.T) {
	release := "1.4.287"
	payload := []byte{0x08, 0x01}
	payload = append(payload, 0x12, byte(len(release)))
	payload = append(payload, []byte(release)...)
	payload = append(payload, 0x1a, 0x05)
	payload = append(payload, []byte("Linux")...)
	payload = append(payload, 0x22, 0x05)
	payload = append(payload, []byte("5.10.")...)

	port := startFakeServer(t, func(conn net.Conn) {
		header := make([]byte, 12)
		binary.BigEndian.PutUint32(header[0:], 1<<16|4<<8|0x1f)
		binary.BigEndian.PutUint32(header[8:], uint32(len(payload)))
		_, _ = conn.Write(append(header, payload...))
	})

	prober := NewProber(2 * time.Second)
	res := prober.probeMumble(context.Background(), "127.0.0.1", port)
	if res == nil || res.Service.Name != "mumble" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.Service.Version != release {
		t.Fatalf("version = %q", res.Service.Version)
	}
	if !strings.Contains(res.Banner, "Linux") {
		t.Fatalf("banner = %q", res.Banner)
	}
}

func TestProbeWinRM(t *testing.T) {
	port := startFakeServer(t, func(conn net.Conn) {
		_, _ = conn.Read(make([]byte, 4096))
		_, _ = conn.Write([]byte("HTTP/1.1 401 Unauthorized\r\nServer: Microsoft-HTTPAPI/2.0\r\nWww-Authenticate: Negotiate\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"))
	})

	prober := NewProber(2 * time.Second)
	res := prober.probeWinRM(context.Background(), "127.0.0.1", port)
	if res == nil || res.Service.Name != "winrm" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.StatusCode != 401 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if res.Apps[0] != "winrm" {
		t.Fatalf("apps = %v", res.Apps)
	}
}

func TestParseNTLMType2WithoutTargetInfo(t *testing.T) {
	blob := make([]byte, 56)
	copy(blob, []byte("NTLMSSP\x00"))
	binary.LittleEndian.PutUint32(blob[8:], 2)
	blob[48] = 6
	blob[49] = 1
	binary.LittleEndian.PutUint16(blob[50:], 7601)

	info := parseNTLMType2(blob)
	if info == nil {
		t.Fatal("expected info")
	}
	if !strings.Contains(info.os, "Windows 7") {
		t.Fatalf("os = %q", info.os)
	}
}

func TestAnalyzeBannerAMQPAndRsync(t *testing.T) {
	prober := NewProber(time.Second)

	amqp := append([]byte("AMQP\x00\x00\x09\x01"), []byte("\x00productRabbitMQ\x07version")...)
	amqp = append(amqp, 'S', 0, 0, 0, 6, '3', '.', '1', '2', '.', '1')
	res := prober.analyzeBanner(5672, amqp)
	if res.Service.Name != "amqp" || res.Service.Product != "RabbitMQ" || res.Service.Version != "3.12.1" {
		t.Fatalf("amqp analyse failed: %+v", res.Service)
	}

	res = prober.analyzeBanner(873, []byte("@RSYNCD: 31.0\n@RSYNCD: OK\n"))
	if res.Service.Name != "rsync" || res.Service.Version != "31.0" {
		t.Fatalf("rsync analyse failed: %+v", res.Service)
	}
}
