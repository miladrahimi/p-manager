package logger

import (
	"errors"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"shadowsocks-manager/internal/config"
	"syscall"
)

type Logger struct {
	config *config.Config
	Engine *zap.Logger
}

// Init initializes the Logger.
func (l *Logger) Init() (err error) {
	level := zap.NewAtomicLevel()
	if err = level.UnmarshalText([]byte(l.config.Logger.Level)); err != nil {
		return fmt.Errorf("logger: invalid level %s, err: %v", l.config.Logger.Level, err)
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
	if err != nil {
		return fmt.Errorf("logger: failed to build, err: %v", err)
	}

	return nil
}

// Shutdown closes the Logger.
func (l *Logger) Shutdown() {
	if err := l.Engine.Sync(); err != nil && !errors.Is(err, syscall.ENOTTY) {
		l.Engine.Error("logger: failed to close", zap.Error(err))
	} else {
		l.Engine.Debug("logger: closed successfully")
	}
}

// New creates a new instance of Logger
func New(c *config.Config) (logger *Logger) {
	return &Logger{config: c, Engine: nil}
}
