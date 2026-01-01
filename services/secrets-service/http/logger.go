package http

import (
	"github.com/MaksimVF/ZB/services/secrets-service/utils"
)

// Logger представляет логгер для HTTP
type Logger struct {
	*utils.Logger
}

// New создает новый логгер для HTTP
func New(serviceName string) *Logger {
	return &Logger{
		Logger: utils.New(serviceName),
	}
}

// InfoHTTP логирует HTTP запрос
func (l *Logger) InfoHTTP(method, path string, statusCode int) {
	l.Info().
		Str("http_method", method).
		Str("http_path", path).
		Int("http_status", statusCode).
		Msg("HTTP request processed")
}

// ErrorHTTP логирует HTTP ошибку
func (l *Logger) ErrorHTTP(method, path string, statusCode int, err error) {
	l.Error().
		Err(err).
		Str("http_method", method).
		Str("http_path", path).
		Int("http_status", statusCode).
		Msg("HTTP request failed")
}
