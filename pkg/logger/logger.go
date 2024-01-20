package logger

import (
	"errors"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"syscall"
)

type Logger struct {
	level  string
	format string
	closer func()
	Engine *zap.Logger
}

func (l *Logger) Init() (err error) {
	level := zap.NewAtomicLevel()
	if err = level.UnmarshalText([]byte(l.level)); err != nil {
		return fmt.Errorf("logger: invalid level %s, err: %v", l.level, err)
	}

	l.Engine, err = zap.Config{
		Level:             level,
		Development:       false,
		Encoding:          "json",
		DisableStacktrace: true,
		DisableCaller:     true,
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stderr"},
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
	l.Engine.Debug(msg, fields...)
}

func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.Engine.Info(msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.Engine.Warn(msg, fields...)
}

func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.Engine.Error(msg, fields...)
}

func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.closer()
	l.Engine.Fatal(msg, fields...)
}

func (l *Logger) With(fields ...zap.Field) *zap.Logger {
	return l.Engine.With(fields...)
}

func (l *Logger) Shutdown() {
	if err := l.Engine.Sync(); err != nil && !errors.Is(err, syscall.ENOTTY) {
		l.Engine.Error("logger: failed to close", zap.Error(err))
	} else {
		l.Engine.Info("logger: closed successfully")
	}
}

func New(level, format string, closer func()) (logger *Logger) {
	return &Logger{Engine: nil, level: level, format: format, closer: closer}
}
