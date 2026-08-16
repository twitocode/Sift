package common

import (
	"encoding/base64"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

func NewLogger(getenv func(string) string, level zapcore.Level) (*zap.Logger, zap.AtomicLevel) {
	atomicLevel := zap.NewAtomicLevelAt(level)

	if getenv("APP_ENV") == "production" {
		cfg := zap.NewProductionConfig()
		cfg.Level = atomicLevel
		return zap.Must(cfg.Build()), atomicLevel
	}

	core := zapcore.NewCore(
		newDevelopmentEncoder(),
		zapcore.Lock(os.Stderr),
		atomicLevel,
	)

	return zap.New(
		core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.Development(),
	), atomicLevel
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

	restText := colorDevelopmentFields(strings.TrimSuffix(rest.String(), "\n"), fields)
	if restText != "" {
		buf.AppendByte(' ')
		buf.AppendString(restText)
	}
	buf.AppendByte('\n')

	return buf, nil
}

func colorDevelopmentFields(encoded string, fields []zapcore.Field) string {
	for _, field := range fields {
		value, color, ok := developmentFieldValue(field)
		if !ok {
			continue
		}

		needle := strconv.Quote(field.Key) + ": " + value
		replacement := strconv.Quote(field.Key) + ": " + colorize(value, color)
		encoded = strings.Replace(encoded, needle, replacement, 1)
	}

	return encoded
}

func developmentFieldValue(field zapcore.Field) (string, string, bool) {
	switch field.Type {
	case zapcore.StringType:
		return strconv.Quote(field.String), "36", true
	case zapcore.BoolType:
		return strconv.FormatBool(field.Integer == 1), "35", true
	case zapcore.Int64Type:
		return strconv.FormatInt(field.Integer, 10), "32", true
	case zapcore.Int32Type:
		return strconv.FormatInt(field.Integer, 10), "32", true
	case zapcore.Int16Type:
		return strconv.FormatInt(field.Integer, 10), "32", true
	case zapcore.Int8Type:
		return strconv.FormatInt(field.Integer, 10), "32", true
	case zapcore.Uint64Type:
		return strconv.FormatUint(uint64(field.Integer), 10), "32", true
	case zapcore.Uint32Type:
		return strconv.FormatUint(uint64(field.Integer), 10), "32", true
	case zapcore.Uint16Type:
		return strconv.FormatUint(uint64(field.Integer), 10), "32", true
	case zapcore.Uint8Type:
		return strconv.FormatUint(uint64(field.Integer), 10), "32", true
	case zapcore.UintptrType:
		return strconv.FormatUint(uint64(field.Integer), 10), "32", true
	case zapcore.Float64Type:
		return strconv.FormatFloat(math.Float64frombits(uint64(field.Integer)), 'g', -1, 64), "32", true
	case zapcore.Float32Type:
		return strconv.FormatFloat(float64(math.Float32frombits(uint32(field.Integer))), 'g', -1, 32), "32", true
	case zapcore.DurationType:
		return strconv.Quote(time.Duration(field.Integer).String()), "33", true
	case zapcore.TimeType, zapcore.TimeFullType:
		return "", "33", false
	case zapcore.BinaryType:
		value, ok := field.Interface.([]byte)
		if !ok {
			return "", "", false
		}
		return strconv.Quote(base64.StdEncoding.EncodeToString(value)), "36", true
	case zapcore.ByteStringType:
		value, ok := field.Interface.([]byte)
		if !ok {
			return "", "", false
		}
		return strconv.Quote(string(value)), "36", true
	case zapcore.ErrorType:
		value, ok := field.Interface.(error)
		if !ok {
			return "", "", false
		}
		return strconv.Quote(value.Error()), "31", true
	default:
		return "", "", false
	}
}

func colorize(value, color string) string {
	return "\x1b[" + color + "m" + value + "\x1b[0m"
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
