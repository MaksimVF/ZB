package grpc

import (
	"time"

	"github.com/MaksimVF/ZB/services/secrets-service/utils"
)

// Logger представляет логгер для gRPC
type Logger struct {
	*utils.Logger
}

// New создает новый логгер для gRPC
func New(serviceName string) *Logger {
	return &Logger{
		Logger: utils.New(serviceName),
	}
}

// InfoWithDuration логирует информацию с измерением времени выполнения
func (l *Logger) InfoWithDuration(operation string, start time.Time, fields map[string]any) {
	duration := time.Since(start)
	fields["duration"] = duration.String()

	event := l.Logger.Info()
	for k, v := range fields {
		event = event.Str(k, v.(string))
	}
	event.Msg(operation + " completed")
}
