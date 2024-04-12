package logger

import (
	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"syscall"
)

type Logger struct {
	e        *zap.Logger
	config   *config.Config
	shutdown chan struct{}
}

func (l *Logger) Init() (err error) {
	level := zap.NewAtomicLevel()
	if err = level.UnmarshalText([]byte(l.config.Logger.Level)); err != nil {
		return errors.Wrapf(err, "invalid log level '%s'", l.config.Logger.Level)
	}

	l.e, err = zap.Config{
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
	l.e.Debug(msg, fields...)
}

func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.e.Info(msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.e.Warn(msg, fields...)
}

func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.e.Error(msg, fields...)
}

func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.e.Error(msg, fields...)
	l.shutdown <- struct{}{}
}

func (l *Logger) With(fields ...zap.Field) *zap.Logger {
	return l.e.With(fields...)
}

func (l *Logger) Close() {
	if err := l.e.Sync(); err != nil && !errors.Is(err, syscall.ENOTTY) {
		l.e.Error("cannot close logger", zap.Error(errors.WithStack(err)))
	} else {
		l.e.Info("logger: closed successfully")
	}
}

func New(config *config.Config, closer chan struct{}) (logger *Logger) {
	return &Logger{e: nil, shutdown: closer, config: config}
}
