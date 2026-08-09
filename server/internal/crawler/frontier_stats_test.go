package crawler

import (
	"testing"

	"github.com/twitocode/sift/internal/common"
)

func TestFrontierStats(t *testing.T) {
	frontier := &FrontierStore{
		bufferQueues: common.NewSafeMap[URL, *BQueue](),
	}
	frontier.bufferQueues.Set("one.example", &BQueue{
		URLs: []URL{"https://one.example/a", "https://one.example/b"},
	})
	frontier.bufferQueues.Set("two.example", &BQueue{
		URLs: []URL{"https://two.example/a"},
	})

	stats := frontier.Stats()

	if stats.HostQueues != 2 {
		t.Fatalf("expected 2 host queues, got %d", stats.HostQueues)
	}
	if stats.PendingURLs != 3 {
		t.Fatalf("expected 3 pending URLs, got %d", stats.PendingURLs)
	}
	if stats.LargestQueue != 2 {
		t.Fatalf("expected largest queue to contain 2 URLs, got %d", stats.LargestQueue)
	}
}
