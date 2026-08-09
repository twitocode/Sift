package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type benchmarkResult struct {
	spiders       int
	delay         int
	elapsed       time.Duration
	requests      int64
	pagesParsed   int64
	fetchFailures int64
	dnsFailures   int64
	heapAllocMB   int64
	err           error
}

type outputCapture struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	mirror io.Writer
}

func (c *outputCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.buffer.Write(p); err != nil {
		return 0, err
	}
	if c.mirror != nil {
		if _, err := c.mirror.Write(p); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (c *outputCapture) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buffer.String()
}

func main() {
	spiderValues := flag.String("spiders", "64,128,192", "comma-separated spider counts")
	delayValues := flag.String("delays", "100,200,500", "comma-separated dispatch delays in milliseconds")
	crawlCount := flag.Int("crawl-count", 20000, "pages to crawl per benchmark case")
	maxURLsPerHost := flag.Int("max-urls-per-host", 50, "maximum queued URLs per host")
	maxHostQueues := flag.Int("max-host-queues", 1000, "maximum host queues")
	maxPendingURLs := flag.Int("max-pending-urls", 20000, "maximum pending URLs")
	timeout := flag.Duration("timeout", 30*time.Minute, "maximum duration for each benchmark case")
	showOutput := flag.Bool("show-output", false, "show crawler logs while each case runs")
	flag.Parse()

	spiders, err := parsePositiveInts(*spiderValues)
	if err != nil {
		exitf("invalid -spiders: %v", err)
	}
	delays, err := parsePositiveInts(*delayValues)
	if err != nil {
		exitf("invalid -delays: %v", err)
	}
	if *crawlCount <= 0 || *maxURLsPerHost <= 0 || *maxHostQueues <= 0 || *maxPendingURLs <= 0 {
		exitf("crawl and queue limits must be positive")
	}

	moduleRoot, err := findModuleRoot()
	if err != nil {
		exitf("find Go module root: %v", err)
	}

	crawlBinary, err := buildCrawler(moduleRoot)
	if err != nil {
		exitf("build crawler: %v", err)
	}
	defer os.Remove(crawlBinary)

	fmt.Printf("Benchmarking %d spider counts × %d dispatch delays\n", len(spiders), len(delays))
	fmt.Printf("crawl_count=%d max_urls_per_host=%d max_host_queues=%d max_pending_urls=%d\n\n",
		*crawlCount, *maxURLsPerHost, *maxHostQueues, *maxPendingURLs)

	results := make([]benchmarkResult, 0, len(spiders)*len(delays))
	for _, spiderCount := range spiders {
		for _, delay := range delays {
			result := runCase(
				crawlBinary,
				moduleRoot,
				spiderCount,
				delay,
				*crawlCount,
				*maxURLsPerHost,
				*maxHostQueues,
				*maxPendingURLs,
				*timeout,
				*showOutput,
			)
			results = append(results, result)

			if result.err != nil {
				fmt.Printf("FAILED spiders=%d delay=%dms: %v\n\n", spiderCount, delay, result.err)
			} else {
				fmt.Printf("Completed spiders=%d delay=%dms in %s\n\n", spiderCount, delay, result.elapsed)
			}
		}
	}

	printResults(results, *crawlCount)
}

func parsePositiveInts(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	values := make([]int, 0, len(parts))

	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value <= 0 {
			return nil, fmt.Errorf("expected positive integers, got %q", raw)
		}
		values = append(values, value)
	}

	return values, nil
}

func findModuleRoot() (string, error) {
	candidates := make([]string, 0, 3)
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd, filepath.Join(cwd, "server"))
	}

	if _, source, _, ok := runtime.Caller(0); ok {
		sourceDir := filepath.Dir(source)
		candidates = append(candidates, filepath.Clean(filepath.Join(sourceDir, "..", "..")))
	}

	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		candidate, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}

		if _, err := os.Stat(filepath.Join(candidate, "go.mod")); err == nil {
			return candidate, nil
		}
	}

	return "", errors.New("go.mod not found; run this command from the repository or server directory")
}

func buildCrawler(moduleRoot string) (string, error) {
	tempFile, err := os.CreateTemp("", "sift-crawl-benchmark-*")
	if err != nil {
		return "", err
	}
	binaryPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		os.Remove(binaryPath)
		return "", err
	}
	if err := os.Remove(binaryPath); err != nil {
		return "", err
	}

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/crawl")
	cmd.Dir = moduleRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(binaryPath)
		return "", err
	}
	return binaryPath, nil
}

func runCase(
	crawlBinary string,
	moduleRoot string,
	spiderCount int,
	delay int,
	crawlCount int,
	maxURLsPerHost int,
	maxHostQueues int,
	maxPendingURLs int,
	timeout time.Duration,
	showOutput bool,
) benchmarkResult {
	result := benchmarkResult{spiders: spiderCount, delay: delay}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, crawlBinary)
	cmd.Dir = moduleRoot
	cmd.Env = overrideEnvironment(os.Environ(), map[string]string{
		"CRAWL_COUNT":        strconv.Itoa(crawlCount),
		"SPIDER_COUNT":       strconv.Itoa(spiderCount),
		"MAX_URLS_PER_HOST":  strconv.Itoa(maxURLsPerHost),
		"MAX_HOST_QUEUES":    strconv.Itoa(maxHostQueues),
		"MAX_PENDING_URLS":   strconv.Itoa(maxPendingURLs),
		"JOB_DISPATCH_DELAY": strconv.Itoa(delay),
	})

	output := &outputCapture{}
	if showOutput {
		output.mirror = os.Stdout
	}
	cmd.Stdout = output
	cmd.Stderr = output

	start := time.Now()
	err := cmd.Run()
	outputText := output.String()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.err = fmt.Errorf("timed out after %s", timeout)
		return result
	}
	if err != nil {
		result.err = fmt.Errorf("%w\n%s", err, strings.TrimSpace(outputText))
		return result
	}

	result.elapsed = parseDuration(outputText, time.Since(start))
	result.requests = parseInt64(outputText, `Total Requests:[[:space:]]+([0-9]+)`)
	result.pagesParsed = parseInt64(outputText, `Pages Parsed:[[:space:]]+([0-9]+)`)
	result.fetchFailures = parseInt64(outputText, `Fetch Failures:[[:space:]]+([0-9]+)`)
	result.dnsFailures = parseInt64(outputText, `DNS Failures:[[:space:]]+([0-9]+)`)
	result.heapAllocMB = parseInt64(outputText, `"heap_alloc_mb":[[:space:]]*([0-9]+)`)

	return result
}

func overrideEnvironment(base []string, overrides map[string]string) []string {
	result := append([]string(nil), base...)
	for key, value := range overrides {
		prefix := key + "="
		replaced := false
		for i, entry := range result {
			if strings.HasPrefix(entry, prefix) {
				result[i] = prefix + value
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, prefix+value)
		}
	}
	return result
}

func parseDuration(output string, fallback time.Duration) time.Duration {
	value := parseString(output, `Time Elapsed:[[:space:]]+([^[:space:]\r\n]+)`)
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseInt64(output, pattern string) int64 {
	value := parseString(output, pattern)
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func parseString(output, pattern string) string {
	match := regexp.MustCompile(pattern).FindStringSubmatch(output)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func printResults(results []benchmarkResult, crawlCount int) {
	successful := make([]benchmarkResult, 0, len(results))
	for _, result := range results {
		if result.err == nil {
			successful = append(successful, result)
		}
	}

	sort.Slice(successful, func(i, j int) bool {
		return successful[i].elapsed < successful[j].elapsed
	})

	fmt.Println("Results ranked by elapsed time")
	fmt.Println("spiders  delay  elapsed     requests  parsed  fetch_failures  dns_failures  heap_alloc_mb  status")
	for _, result := range successful {
		status := "healthy"
		if result.requests == 0 ||
			float64(result.fetchFailures)/float64(result.requests) > 0.05 ||
			result.pagesParsed < int64(float64(crawlCount)*0.95) {
			status = "degraded"
		}
		fmt.Printf("%-8d %-6d %-11s %-9d %-7d %-15d %-13d %-14d %s\n",
			result.spiders,
			result.delay,
			result.elapsed.Round(time.Millisecond),
			result.requests,
			result.pagesParsed,
			result.fetchFailures,
			result.dnsFailures,
			result.heapAllocMB,
			status,
		)
	}

	if len(successful) == 0 {
		fmt.Println("No benchmark cases completed successfully.")
		return
	}

	for _, result := range successful {
		if result.requests > 0 &&
			float64(result.fetchFailures)/float64(result.requests) <= 0.05 &&
			result.pagesParsed >= int64(float64(crawlCount)*0.95) {
			fmt.Printf("\nRecommended: SPIDER_COUNT=%d JOB_DISPATCH_DELAY=%dms\n",
				result.spiders, result.delay)
			return
		}
	}

	fmt.Printf("\nFastest run was degraded: SPIDER_COUNT=%d JOB_DISPATCH_DELAY=%dms\n",
		successful[0].spiders, successful[0].delay)
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
