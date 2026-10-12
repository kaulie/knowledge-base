package kb

import (
	"context"
	"fmt"
	"log"
	"time"
)

type ctxLogKey struct{}

type syncLogger struct {
	now  func() time.Time
	logs []SyncLog
}

func newSyncLogger(now func() time.Time) *syncLogger {
	if now == nil {
		now = time.Now
	}
	return &syncLogger{now: now}
}

func (l *syncLogger) add(level, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	at := l.now().UTC()
	l.logs = append(l.logs, SyncLog{At: at, Level: level, Message: msg})
	log.Printf("knowledge-base sync [%s] %s", level, msg)
}

func (l *syncLogger) entries() []SyncLog {
	if l == nil || l.logs == nil {
		return []SyncLog{}
	}
	out := make([]SyncLog, len(l.logs))
	copy(out, l.logs)
	return out
}

func withSyncLog(ctx context.Context, l *syncLogger) context.Context {
	return context.WithValue(ctx, ctxLogKey{}, l)
}

func loggerFrom(ctx context.Context) *syncLogger {
	if ctx == nil {
		return nil
	}
	l, _ := ctx.Value(ctxLogKey{}).(*syncLogger)
	return l
}

func note(ctx context.Context, level, format string, args ...any) {
	if l := loggerFrom(ctx); l != nil {
		l.add(level, format, args...)
		return
	}
	log.Printf("knowledge-base sync [%s] %s", level, fmt.Sprintf(format, args...))
}
