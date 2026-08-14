package networking

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twitocode/sift/internal/metrics"
	"go.uber.org/zap"
)

func TestResolveLooksUpIPv4WithShortTimeout(t *testing.T) {
	var network string
	var timeout time.Duration

	dc := NewDNSCache(zap.NewNop(), metrics.NewCrawlMetrics(zap.NewNop()))
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
	require.InDelta(t, 500*time.Millisecond, timeout, float64(100*time.Millisecond))
	require.Equal(t, "1.2.3.4", ip.String())
}

func TestResolveDoesNotRetryWhileNegativeCacheIsLive(t *testing.T) {
	calls := 0
	dc := NewDNSCache(zap.NewNop(), metrics.NewCrawlMetrics(zap.NewNop()))
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
	dc := NewDNSCache(zap.NewNop(), metrics.NewCrawlMetrics(zap.NewNop()))
	dc.failed.Set("dead.example", DNSFailTracker{
		expiresAt:    time.Now().Add(-time.Second),
		failureCount: 1,
	})

	_, failed := dc.FailedUntil("dead.example")
	require.False(t, failed)
}
