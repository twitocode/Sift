package crawler

import (
	"context"
	"net"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

type DialerContext func(ctx context.Context, network, addr string) (net.Conn, error)

type DNSCache struct {
	ips      *SafeMap[string, net.IP]
	dnsGroup singleflight.Group

	log *zap.Logger
}

func NewDNSCache(log *zap.Logger) *DNSCache {
	return &DNSCache{ips: NewSafeMap[string, net.IP](), log: log}
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
	v, err, shared := dc.dnsGroup.Do(host, func() (interface{}, error) {
		resolver := &net.Resolver{
      PreferGo: true,
    }
		lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		ips, err := resolver.LookupIP(lookupCtx, "ip", host)
		if err != nil {
			return nil, err
		}
		return ips[0], nil
	})
	if err != nil {
		dc.log.Debug("DNS lookup failed", zap.String("host", host), zap.Error(err))
		return nil, err
	}

	ip := v.(net.IP)
	dc.ips.Set(host, ip)

	if shared {
		dc.log.Debug("suppressed duplicate concurrent DNS lookup", zap.String("host", host))
	}
	return ip, nil
}