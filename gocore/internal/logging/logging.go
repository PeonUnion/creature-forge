package logging

// Package logging wraps uber-go/zap behind a small project-specific API so
// callers never touch zap directly (swap-friendly, consistent formatting).

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Level is a log level: debug < info < warn < error.
type Level string

// Levels.
const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Format is output encoding.
type Format string

// Formats.
const (
	FormatConsole Format = "console"
	FormatJSON    Format = "json"
)

// Config controls a Logger.
type Config struct {
	Level  Level  // debug|info|warn|error
	Format Format // console|json
	// Output: "" (stdout) | "stderr" | file path
	Output string
	// Color enables ANSI colors (console only).
	Color bool
}

func parseLevel(s string) zapcore.Level {
	switch Level(strings.ToLower(strings.TrimSpace(s))) {
	case LevelDebug:
		return zapcore.DebugLevel
	case LevelWarn:
		return zapcore.WarnLevel
	case LevelError:
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// Logger is the project log handle (thin wrapper over zap SugaredLogger).
type Logger struct {
	sugared *zap.SugaredLogger
	z       *zap.Logger
}

// New builds a Logger from cfg (zero cfg → sane defaults).
func New(cfg Config) *Logger {
	if cfg.Level == "" {
		cfg.Level = LevelInfo
	}
	if cfg.Format == "" {
		cfg.Format = FormatConsole
	}
	level := parseLevel(string(cfg.Level))
	var sink zapcore.WriteSyncer = os.Stdout
	if strings.EqualFold(cfg.Output, "stderr") {
		sink = os.Stderr
	} else if cfg.Output != "" {
		f, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			sink = zapcore.AddSync(f)
		} else {
			sink = os.Stderr
		}
	}
	encCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	var encoder zapcore.Encoder
	if cfg.Format == FormatJSON {
		encoder = zapcore.NewJSONEncoder(encCfg)
	} else {
		encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		if !cfg.Color {
			encCfg.EncodeLevel = zapcore.CapitalLevelEncoder
		}
		encoder = zapcore.NewConsoleEncoder(encCfg)
	}
	core := zapcore.NewCore(encoder, sink, zap.NewAtomicLevelAt(level))
	z := zap.New(core, zap.AddCallerSkip(1), zap.AddCaller())
	return &Logger{sugared: z.Sugar(), z: z}
}

// ---- per-instance API ----

// Debug logs at debug level (keysAndValues pairs).
func (l *Logger) Debug(msg string, keysAndValues ...any) { l.sugared.Debugw(msg, keysAndValues...) }

// Info logs at info level.
func (l *Logger) Info(msg string, keysAndValues ...any) { l.sugared.Infow(msg, keysAndValues...) }

// Warn logs at warn level.
func (l *Logger) Warn(msg string, keysAndValues ...any) { l.sugared.Warnw(msg, keysAndValues...) }

// Error logs at error level.
func (l *Logger) Error(msg string, keysAndValues ...any) { l.sugared.Errorw(msg, keysAndValues...) }

// With returns a child logger with fields attached.
func (l *Logger) With(fields ...any) *Logger {
	return &Logger{sugared: l.sugared.With(fields...), z: l.z}
}

// Named returns a child logger tagged with a component name.
func (l *Logger) Named(name string) *Logger {
	return &Logger{sugared: l.sugared.Named(name), z: l.z}
}

// Sync flushes buffered logs.
func (l *Logger) Sync() { _ = l.sugared.Sync() }

// ---- package-level default logger ----

var std = New(Config{})

// SetDefault replaces the package-level logger (e.g. from config).
func SetDefault(l *Logger) { std = l }

// Default returns the package-level logger.
func Default() *Logger { return std }

// Package-level convenience delegating to the default logger.

// Debug logs at debug level via the default logger.
func Debug(msg string, keysAndValues ...any) { std.Debug(msg, keysAndValues...) }

// Info logs at info level via the default logger.
func Info(msg string, keysAndValues ...any) { std.Info(msg, keysAndValues...) }

// Warn logs at warn level via the default logger.
func Warn(msg string, keysAndValues ...any) { std.Warn(msg, keysAndValues...) }

// Error logs at error level via the default logger.
func Error(msg string, keysAndValues ...any) { std.Error(msg, keysAndValues...) }

// With returns a child of the default logger with fields attached.
func With(fields ...any) *Logger { return std.With(fields...) }

// Named returns a child of the default logger tagged with a component name.
func Named(name string) *Logger { return std.Named(name) }
