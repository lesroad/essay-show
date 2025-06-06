package log

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
)

func getLoggerCtx(ctx context.Context) logx.Logger {
	return logx.WithContext(ctx).WithCallerSkip(1)
}

func getLogger() logx.Logger {
	return logx.WithCallerSkip(1)
}

func CtxInfo(ctx context.Context, format string, v ...any) {
	getLoggerCtx(ctx).Infof(format, v...)
}

func Info(format string, v ...any) {
	getLogger().Infof(format, v...)
}

func CtxError(ctx context.Context, format string, v ...any) {
	getLoggerCtx(ctx).Errorf(format, v...)
}

func Error(format string, v ...any) {
	getLogger().Errorf(format, v...)
}

func CtxDebug(ctx context.Context, format string, v ...any) {
	getLoggerCtx(ctx).Debugf(format, v...)
}
