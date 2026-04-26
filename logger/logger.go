package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Config represents configuration details for logging.
type Config struct {
	Filename string `json:"filename"`
	Level    string `json:"level"`
}

// VantageLogger wraps slog.Logger to provide compatibility with legacy logging patterns.
type VantageLogger struct {
	*slog.Logger
}

func (l *VantageLogger) Debug(msg any, args ...any) {
	l.Logger.Debug(msgToString(msg), args...)
}

func (l *VantageLogger) Info(msg any, args ...any) {
	l.Logger.Info(msgToString(msg), args...)
}

func (l *VantageLogger) Warn(msg any, args ...any) {
	l.Logger.Warn(msgToString(msg), args...)
}

func (l *VantageLogger) Error(msg any, args ...any) {
	l.Logger.Error(msgToString(msg), args...)
}

func (l *VantageLogger) Debugf(format string, args ...any) {
	l.Logger.Debug(fmt.Sprintf(format, args...))
}

func (l *VantageLogger) Infof(format string, args ...any) {
	l.Logger.Info(fmt.Sprintf(format, args...))
}

func (l *VantageLogger) Warnf(format string, args ...any) {
	l.Logger.Warn(fmt.Sprintf(format, args...))
}

func (l *VantageLogger) Errorf(format string, args ...any) {
	l.Logger.Error(fmt.Sprintf(format, args...))
}

// Global logger and writer
var Logger *VantageLogger
var logWriter io.Writer = os.Stderr

// Writer returns the current log writer
func Writer() io.Writer {
	return logWriter
}

func init() {
	// Default logger
	Logger = &VantageLogger{
		slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}
}

// Setup configures the logger based on options in the config.json.
func Setup(config *Config) error {
	var level slog.Level
	switch strings.ToLower(config.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var out io.Writer = os.Stderr
	if config.Filename != "" {
		f, err := os.OpenFile(config.Filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		out = io.MultiWriter(os.Stderr, f)
	}

	handler := slog.NewTextHandler(out, &slog.HandlerOptions{
		Level: level,
	})

	logWriter = out
	Logger = &VantageLogger{slog.New(handler)}
	slog.SetDefault(Logger.Logger)

	return nil
}

// Helper methods to maintain compatibility with existing code calling logger.Info, etc.

func msgToString(msg any) string {
	switch v := msg.(type) {
	case string:
		return v
	case error:
		return v.Error()
	default:
		return fmt.Sprint(v)
	}
}

func Debug(msg any, args ...any) {
	Logger.Debug(msg, args...)
}

func Info(msg any, args ...any) {
	Logger.Info(msg, args...)
}

func Warn(msg any, args ...any) {
	Logger.Warn(msg, args...)
}

func Error(msg any, args ...any) {
	Logger.Error(msg, args...)
}

func Fatal(msg any, args ...any) {
	Logger.Error(msg, args...)
	os.Exit(1)
}

// Formatting helpers (legacy support)

func Debugf(format string, args ...any) {
	Logger.Debug(fmt.Sprintf(format, args...))
}

func Infof(format string, args ...any) {
	Logger.Info(fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...any) {
	Logger.Warn(fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...any) {
	Logger.Error(fmt.Sprintf(format, args...))
}

func Fatalf(format string, args ...any) {
	Logger.Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}

// WithFields implements legacy support for logrus-style fields
func WithFields(fields map[string]interface{}) *VantageLogger {
	attrs := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}
	return &VantageLogger{Logger.With(attrs...)}
}

// With returns a logger with context (for task-specific logging)
func With(args ...any) *VantageLogger {
	return &VantageLogger{Logger.With(args...)}
}

func WithContext(ctx context.Context) *VantageLogger {
	return Logger
}

// GormLogger implements the gorm.Logger interface using slog.
type GormLogger struct{}

func (g GormLogger) Print(v ...interface{}) {
	if len(v) < 2 {
		return
	}
	level := v[0]
	if level == "sql" {
		Logger.Debug("SQL Query",
			"duration", v[2],
			"query", v[3],
			"values", v[4],
			"rows", v[5],
		)
	} else if level == "log" {
		Logger.Info("GORM Log", "data", v[2])
	} else {
		Logger.Info("GORM", "data", v)
	}
}
