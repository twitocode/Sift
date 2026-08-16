package networking

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/metrics"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

const dnsLookupTimeout = 500 * time.Millisecond
const maxConcurrentDNSLookups = 64

type DialerContext func(ctx context.Context, network, addr string) (net.Conn, error)
type ipLookup func(ctx context.Context, network, host string) ([]net.IP, error)

type DNSCache struct {
	ips    *common.SafeMap[string, net.IP]
	failed *common.SafeMap[string, DNSFailTracker]
	lookup ipLookup

	dnsGroup   singleflight.Group
	lookupSlot chan struct{}

	metrics *metrics.CrawlMetrics
	log     *zap.Logger
}

type DNSFailTracker struct {
	expiresAt    time.Time
	failureCount int
}

func NewDNSCache(log *zap.Logger, metrics *metrics.CrawlMetrics) *DNSCache {
	return &DNSCache{
		ips:        common.NewSafeMap[string, net.IP](),
		lookup:     systemLookup,
		log:        log,
		metrics:    metrics,
		failed:     common.NewSafeMap[string, DNSFailTracker](),
		lookupSlot: make(chan struct{}, maxConcurrentDNSLookups),
	}
}

func systemLookup(ctx context.Context, network, host string) ([]net.IP, error) {
	resolver := &net.Resolver{PreferGo: true}
	return resolver.LookupIP(ctx, network, host)
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
	if !ok {
		resolved, err := dc.resolve(ctx, host)
		if err != nil {
			return nil, err
		}
		ip = resolved
	}

	return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
}

func (dc *DNSCache) resolve(ctx context.Context, host string) (net.IP, error) {
	if _, failed := dc.FailedUntil(host); failed {
		dc.log.Debug("DNS lookup already failed", zap.String("host", host))
		return nil, errors.New("DNS lookup already failed")
	}

	v, err, shared := dc.dnsGroup.Do(host, func() (interface{}, error) {
		select {
		case dc.lookupSlot <- struct{}{}:
			defer func() { <-dc.lookupSlot }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		lookupCtx, cancel := context.WithTimeout(ctx, dnsLookupTimeout)
		defer cancel()

		ips, err := dc.lookup(lookupCtx, "ip4", host)

		if err != nil {
			tracker, _ := dc.failed.Get(host)
			tracker.failureCount++
			tracker.expiresAt = time.Now().Add(dnsFailureCooldown(tracker.failureCount))
			dc.failed.Set(host, tracker)
			return nil, err
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
