package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger is the global structured logger using log/slog.
var Logger *slog.Logger

// Config represents configuration details for logging.
type Config struct {
	Filename string `json:"filename"`
	Level    string `json:"level"`
}

func init() {
	// Default logger
	Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
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

	// Use JSON handler for production logging if needed, but sticking to Text for now
	// as per Gophish style, just structured.
	handler := slog.NewTextHandler(out, &slog.HandlerOptions{
		Level: level,
	})

	Logger = slog.New(handler)
	slog.SetDefault(Logger) // Set as global default as well

	return nil
}

// Helper methods to maintain compatibility with existing code calling logger.Info, etc.

func Debug(msg string, args ...any) {
	Logger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	Logger.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	Logger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	Logger.Error(msg, args...)
}

func Fatal(msg string, args ...any) {
	Logger.Error(msg, args...)
	os.Exit(1)
}

// Formatting helpers (legacy support)

func Debugf(format string, args ...any) {
	Logger.Debug(strings.TrimSpace(slog.LevelDebug.String()) + ": " + format, args...)
}

func Infof(format string, args ...any) {
	Logger.Info(strings.TrimSpace(slog.LevelInfo.String()) + ": " + format, args...)
}

func Warnf(format string, args ...any) {
	Logger.Warn(strings.TrimSpace(slog.LevelWarn.String()) + ": " + format, args...)
}

func Errorf(format string, args ...any) {
	Logger.Error(strings.TrimSpace(slog.LevelError.String()) + ": " + format, args...)
}

func Fatalf(format string, args ...any) {
	Logger.Error(strings.TrimSpace(slog.LevelError.String()) + ": " + format, args...)
	os.Exit(1)
}

// With returns a logger with context (for task-specific logging)
func With(args ...any) *slog.Logger {
	return Logger.With(args...)
}

func WithContext(ctx context.Context) *slog.Logger {
	return Logger // Could be enhanced to pull trace IDs from context
}

// GormLogger implements the gorm.Logger interface using slog.
type GormLogger struct{}

func (g GormLogger) Print(v ...interface{}) {
	if len(v) < 2 {
		return
	}
	level := v[0]
	if level == "sql" {
		// Log SQL queries at debug level
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
