package scanner

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/expvintl/scopex/internal/util"
)

type OpenPort struct {
	IP    string
	Port  int
	Proto string
}

type ScanStats struct {
	Probed int64
	Open   int64
}

// Native 是内置 TCP connect 扫描器：无需 root，跨平台可用。
type Native struct {
	Concurrency int
	Timeout     time.Duration

	mu      sync.RWMutex
	localIP net.IP
}

// SetLocalIP 指定扫描的源地址（即出口网卡），nil 表示交给系统路由。
func (n *Native) SetLocalIP(ip net.IP) {
	n.mu.Lock()
	n.localIP = ip
	n.mu.Unlock()
}

func (n *Native) LocalIP() net.IP {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.localIP
}

func NewNative(concurrency int, timeout time.Duration) *Native {
	if concurrency <= 0 {
		concurrency = 256
	}
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	return &Native{Concurrency: concurrency, Timeout: timeout}
}

func (n *Native) Scan(ctx context.Context, ranges []util.IPRange, ports []int, rate int, emit func(OpenPort), progress func(done, total int64)) (ScanStats, error) {
	type job struct {
		ip   uint32
		port int
	}
	jobs := make(chan job, n.Concurrency*2)
	stats := ScanStats{}
	var mu sync.Mutex
	var total int64
	for _, r := range ranges {
		total += int64(r.Count()) * int64(len(ports))
	}
	var wg sync.WaitGroup
	dialer := &net.Dialer{Timeout: n.Timeout}
	if ip := n.LocalIP(); ip != nil {
		dialer.LocalAddr = &net.TCPAddr{IP: ip}
	}

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			ipStr := util.Uint32ToIP(j.ip).String()
			conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ipStr, strconv.Itoa(j.port)))
			mu.Lock()
			stats.Probed++
			done := stats.Probed
			if err == nil {
				stats.Open++
			}
			mu.Unlock()
			if progress != nil && done%500 == 0 {
				progress(done, total)
			}
			if err == nil {
				conn.Close()
				emit(OpenPort{IP: ipStr, Port: j.port, Proto: "tcp"})
			}
		}
	}
	for i := 0; i < n.Concurrency; i++ {
		wg.Add(1)
		go worker()
	}

	if rate <= 0 {
		rate = 1000
	}
	start := time.Now()
	var sent int64

feed:
	for _, r := range ranges {
		for ip := r.Start; ; ip++ {
			for _, p := range ports {
				select {
				case <-ctx.Done():
					break feed
				case jobs <- job{ip: ip, port: p}:
				}
				sent++
				if sent%500 == 0 {
					want := time.Duration(float64(sent) / float64(rate) * float64(time.Second))
					if diff := want - time.Since(start); diff > 0 {
						select {
						case <-ctx.Done():
							break feed
						case <-time.After(diff):
						}
					}
				}
			}
			if ip == r.End {
				break
			}
		}
	}
	close(jobs)
	wg.Wait()
	return stats, ctx.Err()
}
