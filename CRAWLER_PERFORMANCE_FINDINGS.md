# Crawler Performance Findings

## Summary

The frontier redesign improves fairness between hosts, but the available crawl data indicates that the frontier is not the primary throughput bottleneck. The crawler is predominantly network-bound, with DNS failures and connection latency occupying nearly all spider workers.

## Evidence

One of the slower milestones reported:

```text
250 pages in 14.27 seconds
192 configured spiders
188 in flight
191 jobs in the ready queue
0 links waiting
0 pages waiting
```

This is approximately 17.5 pages per second while almost every spider is already busy. The scheduler has work available, so increasing scheduling fairness cannot make those active requests complete faster.

The final summary also reported:

```text
Total requests: 31,326
Fetch failures:  8,852
DNS failures:    8,503
Unique hosts:   36,344
```

Approximately 96% of fetch failures were associated with DNS failures.

## Findings

### 1. Failed DNS lookups are not cached

`DNSCache` only stores successful resolutions. A host that fails DNS resolution can therefore be retried for every queued URL. Each lookup can occupy a spider for up to five seconds.

`singleflight` only combines concurrent lookups for the same host. Because the frontier normally dispatches one request per host at a time, repeated sequential failures are not suppressed.

### 2. Network timeouts occupy most workers

The current limits are:

```text
DNS lookup timeout: 5 seconds
Connection timeout: 10 seconds
Whole HTTP request timeout: 15 seconds
```

With 188–190 requests in flight, falling throughput indicates that workers are waiting on DNS, connection establishment, TLS, or response data rather than waiting for frontier work.

### 3. Host counting is accidentally O(n)

`SafeMap.Length()` collects every key into a new slice:

```go
keys := slices.Collect(maps.Keys(sm.m))
return len(keys)
```

This operation is called whenever a new host is added. Creating 36,344 hosts results in roughly 660 million accumulated key visits. It also allocates temporary slices and exclusively locks the map, increasing garbage collection and blocking other host lookups.

### 4. The crawl is extremely broad

The crawler discovered more unique hosts than it fetched pages. Requests to new hosts usually require:

- A DNS lookup
- A new TCP connection
- A TLS handshake
- Little or no connection reuse

Broad crawling is inherently slower than fetching several pages from each host.

### 5. Frontier statistics scan every host

Every 250 fetched pages, `FrontierStore.Stats()` traverses the complete host map and locks each host queue. This becomes progressively more expensive as the crawler approaches `MaxHostQueues`.

This is unlikely to be the primary bottleneck, but it can create increasingly long scheduler pauses.

## Recommended Fixes

### Priority 1: Add negative DNS caching

Cache failed DNS resolutions with a limited TTL, for example:

```text
NXDOMAIN:       10–30 minutes
Temporary error: 30–60 seconds
Timeout:          30–60 seconds
```

Before resolving a host, check both the positive and negative caches. This prevents every URL from repeating the same failed lookup.

Do not permanently cache temporary DNS failures.

### Priority 2: Quarantine repeatedly failing hosts

Track consecutive failures per host. After a threshold, stop dispatching its queued URLs for a longer period:

```text
1 failure: normal cooldown
2 failures: 30 seconds
3 failures: 2 minutes
5 failures: drop or persist remaining URLs
```

Reset the failure count after a successful request. Consider handling DNS failures separately from HTTP status responses.

### Priority 3: Make `SafeMap.Length()` O(1)

Return the built-in map length while holding a read lock:

```go
func (sm *SafeMap[K, V]) Length() int {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    return len(sm.m)
}
```

This removes the repeated key copying, temporary allocations, and exclusive lock.

### Priority 4: Try all resolved IP addresses

The DNS cache currently stores only the first returned IP address. If that address is unreachable, the request fails without trying alternatives.

Store all resolved addresses and try another address when connection establishment fails. Prefer a bounded strategy to avoid multiplying timeout duration.

### Priority 5: Reduce timeout cost carefully

Measure latency before changing timeouts. If the crawler values throughput over coverage, shorter limits may be appropriate:

```text
DNS:       2–3 seconds
Connect:   3–5 seconds
HTTP:     10–12 seconds
```

Shorter timeouts improve throughput but reject more slow websites, so this is a crawl-policy decision rather than an unconditional optimization.

### Priority 6: Avoid full frontier scans for frequent metrics

Maintain inexpensive counters for:

- Active hosts
- Cooling hosts
- Ready hosts
- Pending URLs

Collect expensive statistics such as the largest queue and oldest URL less frequently, or only when diagnostics are enabled.

### Priority 7: Control crawl breadth

Possible policies include:

- Limit newly discovered hosts per page.
- Prioritize hosts that already have successful responses.
- Reserve part of the worker pool for known-good hosts.
- Deprioritize hosts with no successful request history.
- Persist low-priority hosts for a later crawl.

These policies trade breadth for throughput and should match the crawler's product requirements.

## Additional Measurements

Before and after each optimization, record:

```text
request latency p50/p95/p99
DNS latency p50/p95/p99
DNS cache hit rate
negative DNS cache hit rate
connect and TLS failure counts
requests completed per second
workers in DNS/connect/read/parse states
ready-host queue depth
oldest ready-host age
```

Interpretation:

- A full ready queue and nearly all workers in flight indicates a network bottleneck.
- An empty ready queue with idle workers indicates a scheduler/frontier bottleneck.
- A growing link queue indicates host admission or deduplication is too slow.
- A growing page queue indicates parsing or storage is too slow.

## Expected Outcome

The highest-impact combination is:

1. Fix `SafeMap.Length()`.
2. Add negative DNS caching.
3. Quarantine repeatedly failing hosts.
4. Measure request phase latency before tuning timeouts.

The frontier redesign should remain because it improves politeness and fairness, but it should not be expected to reduce DNS or HTTP latency.
