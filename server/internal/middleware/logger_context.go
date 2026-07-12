package middleware

import (
	"context"
	"net/http"

	"go.uber.org/zap"
)

type loggerCtxKey struct{}

func AttachLogger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), loggerCtxKey{}, log)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func Logger(r *http.Request) *zap.Logger {
	log, _ := r.Context().Value(loggerCtxKey{}).(*zap.Logger)
	if log == nil {
		return zap.NewNop()
	}
	return log
}
