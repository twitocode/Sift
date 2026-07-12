package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func LoggingMiddleware(log *zap.Logger, pretty bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			status := ww.Status()
			duration := time.Since(start)

			if pretty {
				log.Info(formatHTTPRequestPretty(r.Method, r.URL.Path, status, duration, r.RemoteAddr))
				return
			}

			log.Info("http request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", status),
				zap.Duration("duration", duration),
				zap.String("ip", r.RemoteAddr),
			)
		})
	}
}

func formatHTTPRequestPretty(method, path string, status int, duration time.Duration, addr string) string {
	return fmt.Sprintf(
		"%s %s %s %s %s",
		coloredHTTPMethod(method),
		path,
		coloredHTTPStatus(status),
		formatHTTPDuration(duration),
		dimText(stripPort(addr)),
	)
}

func stripPort(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

func coloredHTTPMethod(method string) string {
	color := "37"
	switch method {
	case http.MethodGet:
		color = "36"
	case http.MethodPost:
		color = "32"
	case http.MethodPut, http.MethodPatch:
		color = "33"
	case http.MethodDelete:
		color = "31"
	}
	return "\x1b[" + color + "m" + method + "\x1b[0m"
}

func coloredHTTPStatus(status int) string {
	color := "37"
	switch {
	case status >= 500:
		color = "31"
	case status >= 400:
		color = "33"
	case status >= 300:
		color = "36"
	case status >= 200:
		color = "32"
	}
	return "\x1b[" + color + "m" + formatStatus(status) + "\x1b[0m"
}

func formatStatus(status int) string {
	return fmt.Sprintf("%d", status)
}

func formatHTTPDuration(d time.Duration) string {
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%.2fs", d.Seconds())
	case d >= time.Millisecond:
		return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
	default:
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
}

func dimText(text string) string {
	return "\x1b[90m" + text + "\x1b[0m"
}