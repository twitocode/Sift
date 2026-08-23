package networking

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/metrics"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

const dnsFirstLookupTimeout = 2 * time.Second
const dnsRetryLookupTimeout = 500 * time.Millisecond
const maxConcurrentDNSLookups = 256
const prefetchWorkers = 8
const prefetchQueueSize = 2048

var publicDNSServers = []string{"1.1.1.1:53", "8.8.8.8:53"}

type DialerContext func(ctx context.Context, network, addr string) (net.Conn, error)
type ipLookup func(ctx context.Context, network, host string) ([]net.IP, error)
type dnsDialFunc func(ctx context.Context, network, address string) (net.Conn, error)

type DNSCache struct {
	ips     *common.SafeMap[string, net.IP]
	failed  *common.SafeMap[string, DNSFailTracker]
	lookup  ipLookup
	dnsDial dnsDialFunc

	dnsGroup   singleflight.Group
	lookupSlot chan struct{}
	prefetchCh chan string
	dnsRR      atomic.Uint64

	resolver  *net.Resolver
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once

	metrics *metrics.CrawlMetrics
	log     *zap.Logger
}

type DNSFailTracker struct {
	expiresAt    time.Time
	failureCount int
}

func NewDNSCache(log *zap.Logger, metrics *metrics.CrawlMetrics) *DNSCache {
	ctx, cancel := context.WithCancel(context.Background())
	dc := &DNSCache{
		ips:        common.NewSafeMap[string, net.IP](),
		log:        log,
		metrics:    metrics,
		failed:     common.NewSafeMap[string, DNSFailTracker](),
		lookupSlot: make(chan struct{}, maxConcurrentDNSLookups),
		prefetchCh: make(chan string, prefetchQueueSize),
		dnsDial:    defaultDNSDial,
		ctx:        ctx,
		cancel:     cancel,
	}
	dc.resolver = &net.Resolver{
		PreferGo: true,
		Dial:     dc.dialDNS,
	}
	dc.lookup = dc.systemLookup

	for range prefetchWorkers {
		go dc.prefetchLoop()
	}

	return dc
}

func defaultDNSDial(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: dnsFirstLookupTimeout}
	return dialer.DialContext(ctx, network, address)
}

func (dc *DNSCache) systemLookup(ctx context.Context, network, host string) ([]net.IP, error) {
	return dc.resolver.LookupIP(ctx, network, host)
}

func (dc *DNSCache) dialDNS(ctx context.Context, network, _ string) (net.Conn, error) {
	start := int(dc.dnsRR.Add(1) - 1)
	var lastErr error
	for i := range publicDNSServers {
		server := publicDNSServers[(start+i)%len(publicDNSServers)]
		conn, err := dc.dnsDial(ctx, network, server)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (dc *DNSCache) Close() {
	dc.closeOnce.Do(func() {
		dc.cancel()
	})
}

func (dc *DNSCache) Prefetch(ctx context.Context, host string) {
	if host == "" || net.ParseIP(host) != nil {
		return
	}
	if _, ok := dc.ips.Get(host); ok {
		return
	}
	if _, failed := dc.FailedUntil(host); failed {
		return
	}

	select {
	case <-ctx.Done():
	case <-dc.ctx.Done():
	case dc.prefetchCh <- host:
	default:
	}
}

func (dc *DNSCache) prefetchLoop() {
	for {
		select {
		case <-dc.ctx.Done():
			return
		case host := <-dc.prefetchCh:
			_, _ = dc.resolve(dc.ctx, host)
		}
	}
}

func (dc *DNSCache) FailedUntil(host string) (time.Time, bool) {
	tracker, ok := dc.failed.Get(host)
	if !ok || !time.Now().Before(tracker.expiresAt) {
		return time.Time{}, false
	}
	return tracker.expiresAt, true
}

func (dc *DNSCache) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}

	ip, ok := dc.ips.Get(host)
	if ok {
		dc.metrics.DNSCacheHits.Add(1)
	} else {
		resolved, err := dc.resolve(ctx, host)
		if err != nil {
			return nil, err
		}
		ip = resolved
	}

	return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
}

func (dc *DNSCache) resolve(ctx context.Context, host string) (net.IP, error) {
	if parsed := net.ParseIP(host); parsed != nil {
		return parsed, nil
	}

	if ip, ok := dc.ips.Get(host); ok {
		dc.metrics.DNSCacheHits.Add(1)
		return ip, nil
	}

	if _, failed := dc.FailedUntil(host); failed {
		dc.log.Debug("DNS lookup already failed", zap.String("host", host))
		return nil, errors.New("DNS lookup already failed")
	}

	v, err, shared := dc.dnsGroup.Do(host, func() (interface{}, error) {
		if ip, ok := dc.ips.Get(host); ok {
			return ip, nil
		}

		waitStart := time.Now()
		select {
		case dc.lookupSlot <- struct{}{}:
			dc.metrics.DNSSemaphoreWaitNanos.Add(time.Since(waitStart).Nanoseconds())
			dc.metrics.DNSSemaphoreWaits.Add(1)
			defer func() { <-dc.lookupSlot }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		lookupCtx, cancel := context.WithTimeout(ctx, dc.lookupTimeout(host))
		defer cancel()

		lookupStart := time.Now()
		ips, err := dc.lookup(lookupCtx, "ip4", host)
		dc.metrics.DNSLookups.Add(1)
		dc.metrics.DNSLookupNanos.Add(time.Since(lookupStart).Nanoseconds())
		if err != nil {
			return nil, dc.markFailed(host, err)
		}
		if len(ips) == 0 || ips[0] == nil {
			return nil, dc.markFailed(host, errors.New("no A records"))
		}
		return ips[0], nil
	})

	if err != nil {
		dc.metrics.DNSLookupFailures.Add(1)
		dc.log.Debug("DNS lookup failed", zap.String("host", host), zap.Error(err))
		return nil, err
	}

	ip := v.(net.IP)
	dc.ips.Set(host, ip)
	dc.failed.Delete(host)

	if shared {
		dc.log.Debug("suppressed duplicate concurrent DNS lookup", zap.String("host", host))
	}
	return ip, nil
}

func (dc *DNSCache) lookupTimeout(host string) time.Duration {
	tracker, ok := dc.failed.Get(host)
	if ok && tracker.failureCount > 0 {
		return dnsRetryLookupTimeout
	}
	return dnsFirstLookupTimeout
}

func (dc *DNSCache) markFailed(host string, err error) error {
	switch classifyDNSError(err) {
	case dnsFailTimeout:
		dc.metrics.DNSLookupTimeouts.Add(1)
	case dnsFailNXDomain:
		dc.metrics.DNSNXDomain.Add(1)
	}

	tracker, _ := dc.failed.Get(host)
	tracker.failureCount++
	tracker.expiresAt = time.Now().Add(dnsFailureCooldown(tracker.failureCount))
	dc.failed.Set(host, tracker)
	return err
}

type dnsFailKind int

const (
	dnsFailOther dnsFailKind = iota
	dnsFailTimeout
	dnsFailNXDomain
)

func classifyDNSError(err error) dnsFailKind {
	if errors.Is(err, context.DeadlineExceeded) {
		return dnsFailTimeout
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsTimeout {
			return dnsFailTimeout
		}
		if dnsErr.IsNotFound {
			return dnsFailNXDomain
		}
	}

	return dnsFailOther
}

func dnsFailureCooldown(failureCount int) time.Duration {
	switch failureCount {
	case 1:
		return 10 * time.Second
	case 2:
		return 30 * time.Second
	default:
		return 3 * time.Minute
	}
}
