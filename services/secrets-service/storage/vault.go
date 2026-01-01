package storage

import (
	"context"
	"fmt"

	"github.com/hashicorp/vault/api"
)

// VaultStorage реализует хранилище секретов в HashiCorp Vault
type VaultStorage struct {
	client *api.Client
	logger *Logger
}

// NewVaultStorage создает новое хранилище Vault
func NewVaultStorage(client *api.Client, logger *Logger) *VaultStorage {
	return &VaultStorage{
		client: client,
		logger: logger,
	}
}

// GetSecret получает секрет из Vault
func (v *VaultStorage) GetSecret(ctx context.Context, path string) (*api.Secret, error) {
	secret, err := v.client.Logical().Read("secret/data/" + path)
	if err != nil {
		v.logger.Error().
			Err(err).
			Str("path", path).
			Msg("Failed to get secret from Vault")
		return nil, fmt.Errorf("vault read error: %w", err)
	}

	if secret == nil {
		return nil, ErrNotFound("secret %s not found", path)
	}

	return secret, nil
}

// SetSecret сохраняет секрет в Vault
func (v *VaultStorage) SetSecret(ctx context.Context, path string, data map[string]interface{}) error {
	_, err := v.client.Logical().Write("secret/data/"+path, map[string]interface{}{
		"data": data,
	})
	if err != nil {
		v.logger.Error().
			Err(err).
			Str("path", path).
			Msg("Failed to write secret to Vault")
		return fmt.Errorf("vault write error: %w", err)
	}

	return nil
}

// DeleteSecret удаляет секрет из Vault
func (v *VaultStorage) DeleteSecret(ctx context.Context, path string) error {
	_, err := v.client.Logical().Delete("secret/data/" + path)
	if err != nil {
		v.logger.Error().
			Err(err).
			Str("path", path).
			Msg("Failed to delete secret from Vault")
		return fmt.Errorf("vault delete error: %w", err)
	}

	return nil
}

// ListSecrets возвращает список секретов с указанным префиксом
func (v *VaultStorage) ListSecrets(ctx context.Context, prefix string) (*api.Secret, error) {
	secret, err := v.client.Logical().List("secret/metadata/" + prefix)
	if err != nil {
		v.logger.Error().
			Err(err).
			Str("prefix", prefix).
			Msg("Failed to list secrets from Vault")
		return nil, fmt.Errorf("vault list error: %w", err)
	}

	return secret, nil
}

// Health проверяет состояние Vault
func (v *VaultStorage) Health(ctx context.Context) (*api.HealthResponse, error) {
	health, err := v.client.Sys().HealthWithContext(ctx)
	if err != nil {
		v.logger.Error().Err(err).Msg("Vault health check failed")
		return nil, fmt.Errorf("vault health check error: %w", err)
	}

	return health, nil
}

// ErrNotFound возвращает ошибку "не найдено"
func ErrNotFound(format string, args ...any) error {
	return &NotFoundError{Message: fmt.Sprintf(format, args...)}
}

// NotFoundError представляет ошибку "не найдено"
type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string {
	return "not found: " + e.Message
}

// ErrPermissionDenied возвращает ошибку "доступ запрещен"
func ErrPermissionDenied(format string, args ...any) error {
	return &PermissionDeniedError{Message: fmt.Sprintf(format, args...)}
}

// PermissionDeniedError представляет ошибку "доступ запрещен"
type PermissionDeniedError struct {
	Message string
}

func (e *PermissionDeniedError) Error() string {
	return "permission denied: " + e.Message
}
