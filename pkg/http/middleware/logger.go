package middleware

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"time"
)

func Logger(l *logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			start := time.Now()

			err := next(ctx)
			if err != nil {
				ctx.Error(err)
			}

			req := ctx.Request()
			res := ctx.Response()

			fields := []zapcore.Field{
				zap.String("ip", ctx.RealIP()),
				zap.String("latency", time.Since(start).String()),
				zap.String("host", req.Host),
				zap.String("request", fmt.Sprintf("%s %s", req.Method, req.RequestURI)),
				zap.Int("status", res.Status),
				zap.Int64("size", res.Size),
				zap.String("agent", req.UserAgent()),
			}

			id := req.Header.Get(echo.HeaderXRequestID)
			if id == "" {
				id = res.Header().Get(echo.HeaderXRequestID)
				fields = append(fields, zap.String("id", id))
			}

			n := res.Status
			switch {
			case n >= 500:
				l.With(zap.Error(err)).Error("Server error", fields...)
			case n >= 400:
				l.With(zap.Error(err)).Warn("Client error", fields...)
			case n >= 300:
				l.Debug("Redirection", fields...)
			default:
				l.Debug("Success", fields...)
			}

			return nil
		}
	}
}
