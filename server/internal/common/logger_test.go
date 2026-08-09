package common

import (
	"bytes"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestDevelopmentEncoderColorsTypedFields(t *testing.T) {
	var output bytes.Buffer
	core := zapcore.NewCore(
		newDevelopmentEncoder(),
		zapcore.AddSync(&output),
		zapcore.DebugLevel,
	)

	zap.New(core).Info("request",
		zap.String("url", "https://example.com"),
		zap.Int("status", 200),
	)

	logOutput := output.String()
	if !strings.Contains(logOutput, "\x1b[36m\"https://example.com\"\x1b[0m") {
		t.Fatalf("expected string field to be colored, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "\x1b[32m200\x1b[0m") {
		t.Fatalf("expected integer field to be colored, got %q", logOutput)
	}
}

func TestDevelopmentEncoderPreservesMultilineMessages(t *testing.T) {
	var output bytes.Buffer
	core := zapcore.NewCore(
		newDevelopmentEncoder(),
		zapcore.AddSync(&output),
		zapcore.DebugLevel,
	)

	zap.New(core).Info("Crawling Summary\n  URLs Discovered:   500\n  Pages Stored:      498")

	logOutput := output.String()
	if !strings.Contains(logOutput, "Crawling Summary\n  URLs Discovered:   500\n  Pages Stored:      498") {
		t.Fatalf("expected multiline message to be preserved, got %q", logOutput)
	}
}
