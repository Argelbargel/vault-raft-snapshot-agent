package logging

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
)

const (
	FormatDefault = "default"
	FormatText    = "text"
	FormatJson    = "json"

	OutputStdout = "stdout"
	OutputStderr = "stderr"

	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

type logHandlerFactory func(w io.Writer) slog.Handler

var logHandlerFactories = map[string]logHandlerFactory{
	FormatDefault: func(w io.Writer) slog.Handler {
		log.SetOutput(w)
		return slog.Default().Handler()
	},
	FormatText: func(w io.Writer) slog.Handler { return slog.NewTextHandler(w, nil) },
	FormatJson: func(w io.Writer) slog.Handler { return slog.NewJSONHandler(w, nil) },
}

var logLevels = map[string]slog.Level{
	LevelDebug: slog.LevelDebug,
	LevelInfo:  slog.LevelInfo,
	LevelWarn:  slog.LevelWarn,
	LevelError: slog.LevelError,
}

type levelHandler struct {
	level   slog.Level
	handler slog.Handler
}

func (h *levelHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *levelHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.handler.Handle(ctx, r)
}

func (h *levelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &levelHandler{h.level, h.handler.WithAttrs(attrs)}
}

func (h *levelHandler) WithGroup(name string) slog.Handler {
	return &levelHandler{h.level, h.handler.WithGroup(name)}
}

// The actual slog.Logger instance used for logging, initialized with the default for tests.
// We cannot use slog.SetDefault(slog.New(&levelHandler{..., slog.Default().Handler()}) as it causes an infinite loop
// (see https://github.com/golang/go/issues/62424)
var logger = slog.Default()

func Configure(output string, format string, level string) error {
	writer, err := createLogWriter(output)
	if err != nil {
		return err
	}

	handlerFactory, present := logHandlerFactories[format]
	if !present {
		return fmt.Errorf("invalid log-format %s", format)
	}

	logLevel, present := logLevels[level]
	if !present {
		return fmt.Errorf("invalid log-level %s", level)
	}

	logger = slog.New(&levelHandler{logLevel, handlerFactory(writer)})

	return nil
}

func createLogWriter(output string) (io.Writer, error) {
	switch output {
	case OutputStderr:
		return os.Stderr, nil
	case OutputStdout:
		return os.Stdout, nil
	default:
		return os.OpenFile(output, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	}
}

func Debug(msg string, args ...any) {
	logger.Debug(msg, args...)
}

func Error(msg string, args ...any) {
	logger.Error(msg, args...)
}

func Fatal(msg string, args ...any) {
	logger.Error(msg, args...)
	os.Exit(1)
}

func Info(msg string, args ...any) {
	logger.Info(msg, args...)
}

func Panic(msg string, args ...any) {
	Error(msg, args...)
	panic(msg)
}

func Warn(msg string, args ...any) {
	logger.Warn(msg, args...)
}
