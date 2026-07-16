package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	Host                 string   `env:"HOST"`
	Port                 string   `env:"PORT" envDefault:"8000"`
	AppEnv               string   `env:"APP_ENV"`
	DatabaseURL          string   `env:"DATABASE_URL"`
	FrontendAppURL       string   `env:"FRONTEND_APP_URL"`
	SecretKey            string   `env:"SECRET_KEY"`
	InternalServiceToken string   `env:"INTERNAL_SERVICE_TOKEN"`
	AllowedOrigins       []string `env:"ALLOWED_ORIGINS" envSeparator:","`
	OpenRouterAPIKey     string   `env:"OPENROUTER_API_KEY"`
}

func NewConfig(_ func(string) string) *Config {
	_ = godotenv.Load(".env")

	cfg, err := env.ParseAs[Config]()
	if err != nil {
		panic(fmt.Errorf("parse environment config: %w", err))
	}

	if len(cfg.AllowedOrigins) == 0 {
		if cfg.FrontendAppURL != "" {
			cfg.AllowedOrigins = []string{cfg.FrontendAppURL}
		} else {
			cfg.AllowedOrigins = []string{"http://localhost:3000"}
		}
	}

	return &cfg
}

func NewLogger(getenv func(string) string) *zap.Logger {
	if getenv("APP_ENV") == "production" {
		return zap.Must(zap.NewProduction())
	}
	core := zapcore.NewCore(
		newDevelopmentEncoder(),
		zapcore.Lock(os.Stderr),
		zap.NewAtomicLevelAt(zap.DebugLevel),
	)
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel), zap.Development())
}

var logBufferPool = buffer.NewPool()

type levelFirstConsoleEncoder struct {
	zapcore.Encoder
}

func newDevelopmentEncoder() zapcore.Encoder {
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.TimeKey = ""
	cfg.LevelKey = ""
	cfg.NameKey = ""
	cfg.ConsoleSeparator = " "

	return &levelFirstConsoleEncoder{
		Encoder: zapcore.NewConsoleEncoder(cfg),
	}
}

func (e *levelFirstConsoleEncoder) Clone() zapcore.Encoder {
	return &levelFirstConsoleEncoder{Encoder: e.Encoder.Clone()}
}

func (e *levelFirstConsoleEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	rest, err := e.Encoder.EncodeEntry(entry, fields)
	if err != nil {
		return nil, err
	}
	defer rest.Free()

	buf := logBufferPool.Get()
	buf.AppendString(coloredBracketedLevel(entry.Level))
	buf.AppendByte(' ')
	buf.AppendString(entry.Time.Format("15:04:05"))

	restText := strings.TrimSuffix(rest.String(), "\n")
	if restText != "" {
		buf.AppendByte(' ')
		buf.AppendString(restText)
	}
	buf.AppendByte('\n')

	return buf, nil
}

func coloredBracketedLevel(level zapcore.Level) string {
	color := "37"
	switch level {
	case zapcore.DebugLevel:
		color = "35"
	case zapcore.InfoLevel:
		color = "34"
	case zapcore.WarnLevel:
		color = "33"
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		color = "31"
	}
	return "\x1b[" + color + "m[" + level.CapitalString() + "]\x1b[0m"
}