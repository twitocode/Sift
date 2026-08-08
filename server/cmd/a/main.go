package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
)

type CustomHandler struct {
	w io.Writer
}

func NewCustomHandler(w io.Writer) *CustomHandler {
	return &CustomHandler{w: w}
}

func (h *CustomHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *CustomHandler) Handle(_ context.Context, r slog.Record) error {
	levelStr := coloredBracketedLevel(r.Level)
	timeStr := r.Time.Format("15:04:05")

	var sourceStr string
	if r.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		frame, _ := fs.Next()
		if frame.File != "" {
			dir := filepath.Base(filepath.Dir(frame.File))
			file := filepath.Base(frame.File)
			sourceStr = fmt.Sprintf("%s/%s:%d", dir, file, frame.Line)
		}
	}

	attrs := make(map[string]any)
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	var attrsStr string
	if len(attrs) > 0 {
		b, err := json.Marshal(attrs)
		if err == nil {
			attrsStr = " " + string(b)
		}
	}

	_, err := fmt.Fprintf(h.w, "%s %s %s %s%s\n", levelStr, timeStr, sourceStr, r.Message, attrsStr)
	return err
}

func coloredBracketedLevel(level slog.Level) string {
	var color string
	switch {
	case level < slog.LevelInfo:
		color = "35"
	case level < slog.LevelWarn:
		color = "34"
	case level < slog.LevelError:
		color = "33"
	default:
		color = "31"
	}
	return fmt.Sprintf("\x1b[%sm[%s]\x1b[0m", color, level.String())
}

func (h *CustomHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *CustomHandler) WithGroup(name string) slog.Handler {
	return h
}

func main() {
	logger := slog.New(NewCustomHandler(os.Stdout))
	slog.SetDefault(logger)

	slog.Debug("Debugging worker pool")
	slog.Info("Connected to Sqlite")
	slog.Warn("High memory usage detected")
	slog.Error("Failed to write batch")
}