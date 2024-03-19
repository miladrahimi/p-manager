package logger

import (
	"errors"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"syscall"
)

type Logger struct {
	engine   *zap.Logger
	level    string
	format   string
	shutdown chan struct{}
}

func (l *Logger) Init() (err error) {
	level := zap.NewAtomicLevel()
	if err = level.UnmarshalText([]byte(l.level)); err != nil {
		return fmt.Errorf("logger: invalid level %s, err: %v", l.level, err)
	}

	l.engine, err = zap.Config{
		Level:             level,
		Development:       false,
		Encoding:          "json",
		DisableStacktrace: true,
		DisableCaller:     true,
		OutputPaths:       []string{"./storage/logs/app-std.log"},
		ErrorOutputPaths:  []string{"./storage/logs/app-err.log"},
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			EncodeTime:     zapcore.TimeEncoderOfLayout(l.format),
			EncodeDuration: zapcore.StringDurationEncoder,
			LevelKey:       "level",
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			NameKey:        "key",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "message",
			LineEnding:     zapcore.DefaultLineEnding,
		},
	}.Build()
	if err != nil {
		return fmt.Errorf("logger: failed to build, err: %v", err)
	}

	return nil
}

func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.engine.Debug(msg, fields...)
}

func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.engine.Info(msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.engine.Warn(msg, fields...)
}

func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.engine.Error(msg, fields...)
}

func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.engine.Error(msg, fields...)
	l.shutdown <- struct{}{}
}

func (l *Logger) With(fields ...zap.Field) *zap.Logger {
	return l.engine.With(fields...)
}

func (l *Logger) Shutdown() {
	if err := l.engine.Sync(); err != nil && !errors.Is(err, syscall.ENOTTY) {
		l.engine.Error("logger: failed to close", zap.Error(err))
	} else {
		l.engine.Info("logger: closed successfully")
	}
}

func New(level, format string, closer chan struct{}) (logger *Logger) {
	return &Logger{engine: nil, level: level, format: format, shutdown: closer}
}
