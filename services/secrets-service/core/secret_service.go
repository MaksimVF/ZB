package core

import (
	"context"
	"fmt"
	"time"

	"github.com/MaksimVF/ZB/services/secrets-service/config"
	"github.com/MaksimVF/ZB/services/secrets-service/storage"
	"github.com/MaksimVF/ZB/services/secrets-service/utils"
)

// SecretService представляет сервис для работы с секретами
type SecretService struct {
	storage   *storage.VaultStorage
	validator *utils.Validator
	logger    *utils.Logger
	config    *config.Config
}

// NewSecretService создает новый сервис секретов
func NewSecretService(
	storage *storage.VaultStorage,
	validator *utils.Validator,
	logger *utils.Logger,
	config *config.Config,
) *SecretService {
	return &SecretService{
		storage:   storage,
		validator: validator,
		logger:    logger,
		config:    config,
	}
}

// GetSecret получает секрет по имени
func (s *SecretService) GetSecret(ctx context.Context, name string) (string, error) {
	// Валидация
	if err := s.validator.ValidateSecretPath(name); err != nil {
		s.logger.Warn().Err(err).Str("secret_name", name).Msg("Invalid secret path")
		return "", fmt.Errorf("invalid secret path: %w", err)
	}

	// Получение секрета из хранилища
	vaultSecret, err := s.storage.GetSecret(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to get secret: %w", err)
	}

	// Извлечение данных из Vault KV v2
	data, ok := vaultSecret.Data["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid data format in vault response")
	}

	value, ok := data["value"].(string)
	if !ok {
		return "", fmt.Errorf("invalid value format in vault response")
	}

	s.logger.Info().
		Str("secret_name", name).
		Msg("Secret retrieved successfully")

	return value, nil
}

// GetUserSecret получает пользовательский секрет
func (s *SecretService) GetUserSecret(ctx context.Context, userID, secretName string) (string, error) {
	// Валидация
	if err := s.validator.ValidateUserID(userID); err != nil {
		s.logger.Warn().Err(err).Str("user_id", userID).Msg("Invalid user ID")
		return "", fmt.Errorf("invalid user ID: %w", err)
	}

	if err := s.validator.ValidateSecretPath(secretName); err != nil {
		s.logger.Warn().Err(err).Str("secret_name", secretName).Msg("Invalid secret path")
		return "", fmt.Errorf("invalid secret path: %w", err)
	}

	// Получение пользовательского секрета из хранилища
	secretPath := fmt.Sprintf("user-secrets/%s/%s", userID, secretName)
	vaultSecret, err := s.storage.GetSecret(ctx, secretPath)
	if err != nil {
		return "", fmt.Errorf("failed to get user secret: %w", err)
	}

	// Извлечение данных из Vault KV v2
	data, ok := vaultSecret.Data["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid data format in vault response")
	}

	value, ok := data["value"].(string)
	if !ok {
		return "", fmt.Errorf("invalid value format in vault response")
	}

	s.logger.Info().
		Str("user_id", userID).
		Str("secret_name", secretName).
		Msg("User secret retrieved successfully")

	return value, nil
}

// SetUserSecret сохраняет пользовательский секрет
func (s *SecretService) SetUserSecret(ctx context.Context, userID, secretName, secretValue string) error {
	// Валидация
	if err := s.validator.ValidateUserID(userID); err != nil {
		s.logger.Warn().Err(err).Str("user_id", userID).Msg("Invalid user ID")
		return fmt.Errorf("invalid user ID: %w", err)
	}

	if err := s.validator.ValidateSecretPath(secretName); err != nil {
		s.logger.Warn().Err(err).Str("secret_name", secretName).Msg("Invalid secret path")
		return fmt.Errorf("invalid secret path: %w", err)
	}

	if err := s.validator.ValidateSecretValue(secretValue, s.config.MaxSecretValueSize); err != nil {
		s.logger.Warn().Err(err).Str("user_id", userID).Str("secret_name", secretName).Msg("Invalid secret value")
		return fmt.Errorf("invalid secret value: %w", err)
	}

	// Сохранение пользовательского секрета в хранилище
	secretPath := fmt.Sprintf("user-secrets/%s/%s", userID, secretName)
	err := s.storage.SetSecret(ctx, secretPath, map[string]interface{}{
		"value":      secretValue,
		"created_at": time.Now().Unix(),
	})
	if err != nil {
		return fmt.Errorf("failed to save user secret: %w", err)
	}

	s.logger.Info().
		Str("user_id", userID).
		Str("secret_name", secretName).
		Msg("User secret saved successfully")

	return nil
}

// AdminCreateSecret создает секрет через админ API
func (s *SecretService) AdminCreateSecret(ctx context.Context, path, value string) error {
	// Валидация
	if err := s.validator.ValidateSecretPath(path); err != nil {
		s.logger.Warn().Err(err).Str("secret_path", path).Msg("Invalid secret path")
		return fmt.Errorf("invalid secret path: %w", err)
	}

	if err := s.validator.ValidateSecretValue(value, s.config.MaxSecretValueSize); err != nil {
		s.logger.Warn().Err(err).Str("secret_path", path).Msg("Invalid secret value")
		return fmt.Errorf("invalid secret value: %w", err)
	}

	// Сохранение секрета в хранилище
	err := s.storage.SetSecret(ctx, path, map[string]interface{}{
		"value":      value,
		"created_at": time.Now().Unix(),
	})
	if err != nil {
		return fmt.Errorf("failed to save secret: %w", err)
	}

	s.logger.Info().
		Str("secret_path", path).
		Msg("Secret saved successfully")

	return nil
}

// AdminDeleteSecret удаляет секрет через админ API
func (s *SecretService) AdminDeleteSecret(ctx context.Context, path string) error {
	// Валидация
	if err := s.validator.ValidateSecretPath(path); err != nil {
		s.logger.Warn().Err(err).Str("secret_path", path).Msg("Invalid secret path")
		return fmt.Errorf("invalid secret path: %w", err)
	}

	// Удаление секрета из хранилища
	err := s.storage.DeleteSecret(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	s.logger.Info().
		Str("secret_path", path).
		Msg("Secret deleted successfully")

	return nil
}

// AdminListSecrets возвращает список секретов для админ API
func (s *SecretService) AdminListSecrets(ctx context.Context, prefix string) ([]string, error) {
	// Валидация
	if err := s.validator.ValidateSecretPath(prefix); err != nil {
		s.logger.Warn().Err(err).Str("prefix", prefix).Msg("Invalid prefix")
		return nil, fmt.Errorf("invalid prefix: %w", err)
	}

	// Получение списка секретов из хранилища
	vaultSecret, err := s.storage.ListSecrets(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	if vaultSecret == nil || vaultSecret.Data == nil {
		return []string{}, nil
	}

	// Извлечение ключей из ответа Vault
	keysInterface, ok := vaultSecret.Data["keys"]
	if !ok {
		return []string{}, nil
	}

	keys, ok := keysInterface.([]interface{})
	if !ok {
		return []string{}, nil
	}

	// Преобразование в строки
	result := make([]string, len(keys))
	for i, key := range keys {
		if keyStr, ok := key.(string); ok {
			result[i] = keyStr
		}
	}

	s.logger.Info().
		Str("prefix", prefix).
		Int("count", len(result)).
		Msg("Secrets listed successfully")

	return result, nil
}
