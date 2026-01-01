package utils

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

// Logger предоставляет структурированное логирование
type Logger struct {
	zerolog.Logger
}

// New создает новый логгер
func New(serviceName string) *Logger {
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", serviceName).
		Logger()

	return &Logger{Logger: logger}
}

// NewWithWriter создает логгер с указанным writer
func NewWithWriter(serviceName string, writer io.Writer) *Logger {
	logger := zerolog.New(writer).With().
		Timestamp().
		Str("service", serviceName).
		Logger()

	return &Logger{Logger: logger}
}

// WithFields добавляет поля к логгеру
func (l *Logger) WithFields(fields map[string]any) *Logger {
	zerologFields := make(map[string]any, len(fields))
	for k, v := range fields {
		zerologFields[k] = v
	}

	newLogger := l.Logger.With().Fields(zerologFields).Logger()
	return &Logger{Logger: newLogger}
}

// Error создает лог ошибки
func (l *Logger) Error() *zerolog.Event {
	return l.Logger.Error()
}

// Info создает лог информации
func (l *Logger) Info() *zerolog.Event {
	return l.Logger.Info()
}

// Warn создает лог предупреждения
func (l *Logger) Warn() *zerolog.Event {
	return l.Logger.Warn()
}

// Debug создает лог отладки
func (l *Logger) Debug() *zerolog.Event {
	return l.Logger.Debug()
}

// Fatal создает лог фатальной ошибки и завершает программу
func (l *Logger) Fatal() *zerolog.Event {
	return l.Logger.Fatal()
}
