package models

import (
	"time"
)

// Secret представляет структуру секрета
type Secret struct {
	Name      string            `json:"name"`
	Value     string            `json:"value"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// UserSecret представляет секрет, связанный с пользователем
type UserSecret struct {
	UserID     string    `json:"user_id"`
	SecretName string    `json:"secret_name"`
	Value      string    `json:"value"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// SecretList представляет список секретов
type SecretList struct {
	Keys []string `json:"keys"`
}

// SecretOperationResult представляет результат операции с секретом
type SecretOperationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}
