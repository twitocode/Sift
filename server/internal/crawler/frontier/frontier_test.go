package frontier

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/crawler/dedup"
	"github.com/twitocode/sift/internal/metrics"
	"go.uber.org/zap"
)

type stubDNSCache struct {
	prefetched []string
}

func (s *stubDNSCache) FailedUntil(host string) (time.Time, bool) {
	return time.Time{}, false
}

func (s *stubDNSCache) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return nil, errors.New("unused")
}

func (s *stubDNSCache) Prefetch(ctx context.Context, host string) {
	s.prefetched = append(s.prefetched, host)
}

func newTestFrontier(stub *stubDNSCache) *FrontierStore {
	return &FrontierStore{
		hosts:         common.NewSafeMap[common.URL, *HostState](),
		readyHosts:    NewHostQueue(),
		seenURLs:      common.NewSafeMap[common.URL, struct{}](),
		crawledURLs:   common.NewSafeMap[common.URL, struct{}](),
		bloomFilter:   dedup.NewBloomFilter(100, 0.01),
		wakeScheduler: make(chan struct{}, 1),
		cooldownHosts: NewCooldownHeap(10),
		dnsCache:      stub,
		cfg: &common.Config{
			MaxHostQueues:  10,
			MaxPendingURLs: 100,
			MaxURLsPerHost: 10,
		},
		metrics:   metrics.NewCrawlMetrics(zap.NewNop()),
		log:       zap.NewNop(),
		blacklist: map[string]struct{}{},
	}
}

func TestAddOrRetrieveHostDoesNotPrefetchDNS(t *testing.T) {
	stub := &stubDNSCache{}
	fs := newTestFrontier(stub)

	_, err := fs.AddOrRetrieveHost(context.Background(), "https://example.com/page")
	require.NoError(t, err)
	require.Empty(t, stub.prefetched)
}

func TestAddLinkPrefetchesDNSWhenHostIsScheduled(t *testing.T) {
	stub := &stubDNSCache{}
	fs := newTestFrontier(stub)
	ctx := context.Background()

	host, err := fs.AddOrRetrieveHost(ctx, "https://example.com/page")
	require.NoError(t, err)
	require.Empty(t, stub.prefetched)

	fs.AddLink(ctx, host, "https://example.com/page")
	require.Equal(t, []string{"example.com"}, stub.prefetched)
	require.Equal(t, host, fs.readyHosts.Pop())

	fs.AddLink(ctx, host, "https://example.com/other")
	require.Equal(t, []string{"example.com"}, stub.prefetched)
}

func TestFreeExpiredHostsPrefetchesDNS(t *testing.T) {
	stub := &stubDNSCache{}
	fs := newTestFrontier(stub)
	host := &HostState{
		Host:           "example.com",
		URLs:           []common.URL{"https://example.com/a"},
		NextEligibleAt: time.Now().Add(-time.Second),
	}
	fs.cooldownHosts.Add(host)

	fs.FreeExpiredHosts(time.Now())

	require.Equal(t, []string{"example.com"}, stub.prefetched)
	require.Equal(t, host, fs.readyHosts.Pop())
}
