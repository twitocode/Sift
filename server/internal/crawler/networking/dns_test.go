package networking

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twitocode/sift/internal/metrics"
	"go.uber.org/zap"
)

func newTestCache(t *testing.T) *DNSCache {
	t.Helper()
	dc := NewDNSCache(zap.NewNop(), metrics.NewCrawlMetrics(zap.NewNop()))
	t.Cleanup(dc.Close)
	return dc
}

func TestResolveLooksUpIPv4WithFirstLookupTimeout(t *testing.T) {
	var network string
	var timeout time.Duration

	dc := newTestCache(t)
	dc.lookup = func(ctx context.Context, netw, host string) ([]net.IP, error) {
		network = netw
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		timeout = time.Until(deadline)
		return []net.IP{net.ParseIP("1.2.3.4")}, nil
	}

	ip, err := dc.resolve(context.Background(), "example.com")
	require.NoError(t, err)
	require.Equal(t, "ip4", network)
	require.InDelta(t, float64(2*time.Second), float64(timeout), float64(200*time.Millisecond))
	require.Equal(t, "1.2.3.4", ip.String())
}

func TestResolveUsesShortTimeoutOnRetryAfterNegativeCacheExpires(t *testing.T) {
	var timeout time.Duration

	dc := newTestCache(t)
	dc.failed.Set("example.com", DNSFailTracker{
		expiresAt:    time.Now().Add(-time.Second),
		failureCount: 1,
	})
	dc.lookup = func(ctx context.Context, netw, host string) ([]net.IP, error) {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		timeout = time.Until(deadline)
		return []net.IP{net.ParseIP("1.2.3.4")}, nil
	}

	_, err := dc.resolve(context.Background(), "example.com")
	require.NoError(t, err)
	require.InDelta(t, float64(500*time.Millisecond), float64(timeout), float64(100*time.Millisecond))
}

func TestResolveDoesNotRetryWhileNegativeCacheIsLive(t *testing.T) {
	calls := 0
	dc := newTestCache(t)
	dc.lookup = func(ctx context.Context, network, host string) ([]net.IP, error) {
		calls++
		return nil, errors.New("no such host")
	}

	_, err := dc.resolve(context.Background(), "dead.example")
	require.Error(t, err)
	require.Equal(t, 1, calls)

	_, err = dc.resolve(context.Background(), "dead.example")
	require.Error(t, err)
	require.Equal(t, 1, calls)

	until, failed := dc.FailedUntil("dead.example")
	require.True(t, failed)
	require.True(t, until.After(time.Now()))
}

func TestFailedUntilClearsAfterExpiry(t *testing.T) {
	dc := newTestCache(t)
	dc.failed.Set("dead.example", DNSFailTracker{
		expiresAt:    time.Now().Add(-time.Second),
		failureCount: 1,
	})

	_, failed := dc.FailedUntil("dead.example")
	require.False(t, failed)
}

func TestResolveReturnsCachedIPWithoutLookingUp(t *testing.T) {
	calls := 0
	dc := newTestCache(t)
	dc.ips.Set("example.com", net.ParseIP("1.2.3.4"))
	dc.lookup = func(ctx context.Context, network, host string) ([]net.IP, error) {
		calls++
		return []net.IP{net.ParseIP("9.9.9.9")}, nil
	}

	ip, err := dc.resolve(context.Background(), "example.com")
	require.NoError(t, err)
	require.Equal(t, "1.2.3.4", ip.String())
	require.Equal(t, 0, calls)
	require.Equal(t, int64(1), dc.metrics.DNSCacheHits.Load())
	require.Equal(t, int64(0), dc.metrics.DNSLookups.Load())
}

func TestPrefetchResolvesHostBeforeDial(t *testing.T) {
	var calls atomic.Int64
	dc := newTestCache(t)
	dc.lookup = func(ctx context.Context, network, host string) ([]net.IP, error) {
		calls.Add(1)
		require.Equal(t, "example.com", host)
		return []net.IP{net.ParseIP("1.2.3.4")}, nil
	}

	dc.Prefetch(context.Background(), "example.com")

	require.Eventually(t, func() bool {
		ip, ok := dc.ips.Get("example.com")
		return ok && ip.String() == "1.2.3.4"
	}, time.Second, 5*time.Millisecond)

	require.Equal(t, int64(1), calls.Load())

	ip, err := dc.resolve(context.Background(), "example.com")
	require.NoError(t, err)
	require.Equal(t, "1.2.3.4", ip.String())
	require.Equal(t, int64(1), calls.Load())
}

func TestPrefetchSkipsIPHosts(t *testing.T) {
	calls := 0
	dc := newTestCache(t)
	dc.lookup = func(ctx context.Context, network, host string) ([]net.IP, error) {
		calls++
		return []net.IP{net.ParseIP("1.2.3.4")}, nil
	}

	dc.Prefetch(context.Background(), "8.8.8.8")
	time.Sleep(20 * time.Millisecond)
	require.Equal(t, 0, calls)
}

func TestResolveClassifiesTimeoutsAndNXDomain(t *testing.T) {
	dc := newTestCache(t)
	dc.lookup = func(ctx context.Context, network, host string) ([]net.IP, error) {
		if host == "slow.example" {
			return nil, context.DeadlineExceeded
		}
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}

	_, err := dc.resolve(context.Background(), "slow.example")
	require.Error(t, err)
	require.Equal(t, int64(1), dc.metrics.DNSLookupTimeouts.Load())
	require.Equal(t, int64(0), dc.metrics.DNSNXDomain.Load())

	dc.failed.Delete("missing.example")
	_, err = dc.resolve(context.Background(), "missing.example")
	require.Error(t, err)
	require.Equal(t, int64(1), dc.metrics.DNSNXDomain.Load())
	require.Equal(t, int64(1), dc.metrics.DNSLookupTimeouts.Load())
}

func TestResolveRecordsSemaphoreWait(t *testing.T) {
	dc := newTestCache(t)
	dc.lookup = func(ctx context.Context, network, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("1.2.3.4")}, nil
	}

	for i := 0; i < maxConcurrentDNSLookups; i++ {
		dc.lookupSlot <- struct{}{}
	}

	done := make(chan struct{})
	go func() {
		time.Sleep(40 * time.Millisecond)
		<-dc.lookupSlot
		close(done)
	}()

	_, err := dc.resolve(context.Background(), "example.com")
	require.NoError(t, err)
	<-done
	require.Greater(t, dc.metrics.DNSSemaphoreWaitNanos.Load(), int64(20*time.Millisecond))
	require.Equal(t, int64(1), dc.metrics.DNSSemaphoreWaits.Load())
}

func TestDialDNSTriesPublicResolvers(t *testing.T) {
	dc := newTestCache(t)
	var dialed []string
	dc.dnsDial = func(ctx context.Context, network, address string) (net.Conn, error) {
		dialed = append(dialed, address)
		return nil, errors.New("refused")
	}

	_, err := dc.dialDNS(context.Background(), "udp", "ignored:53")
	require.Error(t, err)
	require.Equal(t, []string{"1.1.1.1:53", "8.8.8.8:53"}, dialed)
}

func TestLookupSlotCapacity(t *testing.T) {
	dc := newTestCache(t)
	require.Equal(t, 256, cap(dc.lookupSlot))
}
