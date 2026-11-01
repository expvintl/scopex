package fingerprint

import (
	"context"
	"io"
	"net"
	"strconv"
	"time"
)

// dialProbe 建立带截止时间的探测连接，超时取 min(上下文截止, Prober 超时)。
func (p *Prober) dialProbe(ctx context.Context, ip string, port int) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: p.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(p.Timeout))
	}
	return conn, nil
}

// readAtMost 最多读取 n 个字节，读不满时返回已读部分。
func readAtMost(r io.Reader, n int) []byte {
	if n <= 0 {
		return nil
	}
	buf := make([]byte, n)
	got, err := io.ReadFull(r, buf)
	if err != nil && got == 0 {
		return nil
	}
	return buf[:got]
}
