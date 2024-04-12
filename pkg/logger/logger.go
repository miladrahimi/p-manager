package logger

import (
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"syscall"
)

type Logger struct {
	engine   *zap.Logger
	shutdown chan struct{}
	config   *config.Config
}

func (l *Logger) Init() (err error) {
	level := zap.NewAtomicLevel()
	if err = level.UnmarshalText([]byte(l.config.Logger.Level)); err != nil {
		return fmt.Errorf("logger: invalid level %s, err: %v", l.config.Logger.Level, err)
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
			EncodeTime:     zapcore.TimeEncoderOfLayout(l.config.Logger.Format),
			EncodeDuration: zapcore.StringDurationEncoder,
			LevelKey:       "level",
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			NameKey:        "key",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "message",
			LineEnding:     zapcore.DefaultLineEnding,
		},
	}.Build()
	return errors.Wrap(err, "cannot build logger")
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

func (l *Logger) Close() {
	if err := l.engine.Sync(); err != nil && !errors.Is(err, syscall.ENOTTY) {
		l.engine.Error("cannot close logger", zap.Error(errors.WithStack(err)))
	} else {
		l.engine.Info("logger: closed successfully")
	}
}

func New(config *config.Config, closer chan struct{}) (logger *Logger) {
	return &Logger{engine: nil, shutdown: closer, config: config}
}
