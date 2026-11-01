package fingerprint

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"unicode/utf16"

	"github.com/expvintl/scopex/internal/model"
)

var smb2Dialects = []uint16{0x0202, 0x0210, 0x0300, 0x0302, 0x0311}

var errNotSMB = errors.New("not a smb response")

func smbDialectName(dialect uint16) string {
	switch dialect {
	case 0x0202:
		return "2.0.2"
	case 0x0210:
		return "2.1"
	case 0x02ff:
		return "2.0"
	case 0x0300:
		return "3.0"
	case 0x0302:
		return "3.0.2"
	case 0x0311:
		return "3.1.1"
	}
	return ""
}

// probeSMB 通过 SMB2 协商与 NTLMSSP 挑战获取 SMB 版本、主机名、域与系统版本。
func (p *Prober) probeSMB(ctx context.Context, ip string, port int) *Result {
	conn, err := p.dialProbe(ctx, ip, port)
	if err != nil {
		return nil
	}
	defer conn.Close()

	if _, err := conn.Write(smbNegotiatePacket()); err != nil {
		return nil
	}
	negotiate, err := readSMB2Message(conn)
	if err != nil || len(negotiate) < 40 {
		return nil
	}
	if !bytes.HasPrefix(negotiate, []byte{0xfe, 'S', 'M', 'B'}) {
		return smb1Result(negotiate)
	}
	if len(negotiate) < 72 {
		return nil
	}

	dialect := binary.LittleEndian.Uint16(negotiate[68:70])
	version := smbDialectName(dialect)

	info := p.smbSessionInfo(conn, negotiate)

	res := &Result{Headers: map[string]string{}}
	res.Service = model.Service{
		Name:       "smb",
		Product:    "Microsoft Windows SMB",
		Version:    version,
		Method:     "smb",
		Confidence: 95,
	}
	if info != nil && info.samba {
		res.Service.Product = "Samba"
	}
	if info != nil && info.os != "" {
		res.Service.ExtraInfo = info.os
		if info.samba {
			res.Service.Product = "Samba"
			res.OS = "Linux"
		} else {
			res.Device = info.os
			res.OS = info.os
		}
	} else if info != nil && info.samba {
		res.OS = "Linux"
	}

	var parts []string
	if version != "" {
		parts = append(parts, "SMB "+version)
	}
	if info != nil {
		if info.domain != "" {
			parts = append(parts, "域 "+info.domain)
			res.Domains = append(res.Domains, info.domain)
		}
		if info.host != "" {
			parts = append(parts, "主机 "+info.host)
			res.Hostnames = append(res.Hostnames, info.host)
		}
		if info.dnsDomain != "" && info.dnsDomain != info.domain {
			res.Domains = append(res.Domains, info.dnsDomain)
		}
		if info.dnsHost != "" && info.dnsHost != info.host {
			res.Hostnames = append(res.Hostnames, info.dnsHost)
		}
	}
	res.Banner = strings.Join(parts, " · ")
	res.Service.Banner = res.Banner
	res.Apps = []string{"smb"}
	if info == nil || !info.samba {
		res.CPE = []string{"cpe:/o:microsoft:windows"}
	}
	return res
}

func smb1Result(raw []byte) *Result {
	if len(raw) < 35 {
		return nil
	}
	dialect := binary.LittleEndian.Uint16(raw[33:35])
	version := "1.x"
	if dialect <= 5 {
		version = "1.0"
	}
	return &Result{
		Service: model.Service{
			Name:       "smb",
			Product:    "Microsoft Windows SMB",
			Version:    version,
			Method:     "smb",
			Confidence: 90,
			Banner:     "SMB " + version + "（不支持 SMB2+）",
		},
		Banner:  "SMB " + version + "（不支持 SMB2+）",
		Apps:    []string{"smb"},
		Headers: map[string]string{},
	}
}

func smbNegotiatePacket() []byte {
	body := make([]byte, 0, 36+len(smb2Dialects)*2)
	body = binary.LittleEndian.AppendUint16(body, 36)
	body = binary.LittleEndian.AppendUint16(body, uint16(len(smb2Dialects)))
	body = binary.LittleEndian.AppendUint16(body, 1)
	body = binary.LittleEndian.AppendUint16(body, 0)
	body = binary.LittleEndian.AppendUint32(body, 0)
	body = append(body, make([]byte, 16)...)
	for _, dialect := range smb2Dialects {
		body = binary.LittleEndian.AppendUint16(body, dialect)
	}
	return append(smb2Header(0x0000, nil), body...)
}

func smb2Header(command uint16, sessionID []byte) []byte {
	header := make([]byte, 64)
	copy(header, []byte{0xfe, 'S', 'M', 'B'})
	binary.LittleEndian.PutUint16(header[4:], 64)
	binary.LittleEndian.PutUint16(header[12:], command)
	binary.LittleEndian.PutUint16(header[14:], 1)
	binary.LittleEndian.PutUint64(header[24:], 1)
	copy(header[40:], sessionID)
	return header
}

// readSMB2Message 读取一条完整的 SMB 响应：64 字节头 + 按命令长度读取正文。
func readSMB2Message(conn net.Conn) ([]byte, error) {
	header := readAtMost(conn, 64)
	if len(header) < 4 {
		return nil, errNotSMB
	}
	if bytes.HasPrefix(header, []byte{0xff, 'S', 'M', 'B'}) {
		return header, nil
	}
	if !bytes.HasPrefix(header, []byte{0xfe, 'S', 'M', 'B'}) || len(header) < 64 {
		return nil, errNotSMB
	}

	switch binary.LittleEndian.Uint16(header[12:14]) {
	case 0x0000:
		body := readAtMost(conn, 64)
		if len(body) < 60 {
			return header, nil
		}
		if extra := readAtMost(conn, int(binary.LittleEndian.Uint16(body[58:60]))); extra != nil {
			body = append(body, extra...)
		}
		return append(header, body...), nil
	case 0x0001:
		body := readAtMost(conn, 8)
		if len(body) < 8 {
			return header, nil
		}
		if extra := readAtMost(conn, int(binary.LittleEndian.Uint16(body[6:8]))); extra != nil {
			body = append(body, extra...)
		}
		return append(header, body...), nil
	}
	return header, nil
}

type smbInfo struct {
	host      string
	domain    string
	dnsHost   string
	dnsDomain string
	os        string
	samba     bool
}

// smbSessionInfo 发起一次 NTLMSSP 协商，从挑战中提取主机名与系统版本。
func (p *Prober) smbSessionInfo(conn net.Conn, negotiate []byte) *smbInfo {
	if len(negotiate) < 48 {
		return nil
	}
	sessionID := negotiate[40:48]
	token := ntlmType1()

	body := make([]byte, 24+len(token))
	binary.LittleEndian.PutUint16(body[0:], 25)
	body[3] = 1
	binary.LittleEndian.PutUint16(body[12:], uint16(64+24))
	binary.LittleEndian.PutUint16(body[14:], uint16(len(token)))
	copy(body[24:], token)

	if _, err := conn.Write(append(smb2Header(0x0001, sessionID), body...)); err != nil {
		return nil
	}
	resp, err := readSMB2Message(conn)
	if err != nil || len(resp) < 72 {
		return nil
	}
	offset := int(binary.LittleEndian.Uint16(resp[68:70]))
	length := int(binary.LittleEndian.Uint16(resp[70:72]))
	if length == 0 || offset+length > len(resp) {
		return nil
	}
	return parseNTLMType2(resp[offset : offset+length])
}

func ntlmType1() []byte {
	token := make([]byte, 32)
	copy(token, []byte("NTLMSSP\x00"))
	binary.LittleEndian.PutUint32(token[8:], 1)
	binary.LittleEndian.PutUint32(token[12:], 0xE2088297)
	return token
}

func parseNTLMType2(blob []byte) *smbInfo {
	if len(blob) < 48 || !bytes.HasPrefix(blob, []byte("NTLMSSP\x00")) {
		return nil
	}
	if binary.LittleEndian.Uint32(blob[8:12]) != 2 {
		return nil
	}
	info := &smbInfo{samba: bytes.Contains(bytes.ToLower(blob), []byte("samba"))}

	if len(blob) >= 56 {
		if major, minor := blob[48], blob[49]; major > 0 {
			build := int(binary.LittleEndian.Uint16(blob[50:52]))
			info.os = windowsVersionName(int(major), int(minor), build)
		}
	}

	targetLen := int(binary.LittleEndian.Uint16(blob[40:42]))
	targetOffset := int(binary.LittleEndian.Uint32(blob[44:48]))
	if targetLen == 0 || targetOffset+targetLen > len(blob) {
		return info
	}
	av := blob[targetOffset : targetOffset+targetLen]
	for len(av) >= 4 {
		id := binary.LittleEndian.Uint16(av[0:2])
		size := int(binary.LittleEndian.Uint16(av[2:4]))
		av = av[4:]
		if id == 0 || size > len(av) {
			break
		}
		value := utf16le(av[:size])
		av = av[size:]
		switch id {
		case 1:
			info.host = value
		case 2:
			info.domain = value
		case 3:
			info.dnsHost = value
		case 4:
			info.dnsDomain = value
		}
		if strings.Contains(strings.ToLower(value), "samba") {
			info.samba = true
		}
	}
	if info.host == "" {
		info.host = info.dnsHost
	}
	if info.domain == "" {
		info.domain = info.dnsDomain
	}
	return info
}

func utf16le(raw []byte) string {
	if len(raw)%2 != 0 {
		raw = raw[:len(raw)-1]
	}
	units := make([]uint16, 0, len(raw)/2)
	for index := 0; index+1 < len(raw); index += 2 {
		units = append(units, binary.LittleEndian.Uint16(raw[index:]))
	}
	return strings.TrimSpace(string(utf16.Decode(units)))
}

func windowsVersionName(major, minor, build int) string {
	return windowsBuildName(major, minor, build)
}
