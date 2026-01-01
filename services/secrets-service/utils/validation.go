package utils

import (
	"regexp"
	"strings"
)

// Validator предоставляет функции валидации
type Validator struct{}

// ValidateSecretPath проверяет путь к секрету
func (v *Validator) ValidateSecretPath(path string) error {
	if path == "" {
		return ErrValidation("path cannot be empty")
	}

	// Проверка на паттерны path traversal
	dangerousPatterns := []string{"..", "~", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, pattern := range dangerousPatterns {
		if strings.Contains(path, pattern) {
			return ErrValidation("path contains forbidden character sequence: " + pattern)
		}
	}

	// Валидация формата пути - только буквенно-цифровые, дефисы, подчеркивания и слеши
	pathRegex := regexp.MustCompile(`^[a-zA-Z0-9\-_/]+$`)
	if !pathRegex.MatchString(path) {
		return ErrValidation("path contains invalid characters: only alphanumeric, dashes, underscores, and slashes allowed")
	}

	// Проверка длины пути
	if len(path) > 256 {
		return ErrValidation("path too long: maximum 256 characters allowed")
	}

	// Убедиться, что путь не начинается и не заканчивается слешем
	if strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return ErrValidation("path cannot start or end with slash")
	}

	return nil
}

// ValidateUserID проверяет формат ID пользователя
func (v *Validator) ValidateUserID(userID string) error {
	if userID == "" {
		return ErrValidation("user ID cannot be empty")
	}

	// Валидация формата UUID
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	if !uuidRegex.MatchString(userID) {
		return ErrValidation("invalid user ID format: must be a valid UUID")
	}

	return nil
}

// ValidateAdminKey проверяет формат админ ключа
func (v *Validator) ValidateAdminKey(adminKey string, regex *regexp.Regexp) error {
	if adminKey == "" {
		return ErrValidation("admin key cannot be empty")
	}

	if !regex.MatchString(adminKey) {
		return ErrValidation("invalid admin key format: must be 16-64 chars, alphanumeric with dashes/underscores")
	}

	return nil
}

// ValidateSecretValue проверяет значение секрета
func (v *Validator) ValidateSecretValue(value string, maxSize int) error {
	if value == "" {
		return ErrValidation("secret value cannot be empty")
	}

	if len(value) > maxSize {
		return ErrValidation("secret value too long: maximum %d characters allowed", maxSize)
	}

	return nil
}

// IsOriginAllowed проверяет разрешен ли origin
func (v *Validator) IsOriginAllowed(origin, allowedOrigins string) bool {
	if origin == "" || allowedOrigins == "" {
		return false
	}

	// Разделить разрешенные origins и проверить наличие запрашиваемого
	for _, allowedOrigin := range strings.Split(allowedOrigins, ",") {
		if strings.TrimSpace(allowedOrigin) == origin {
			return true
		}
	}
	return false
}

// ErrValidation возвращает ошибку валидации
func ErrValidation(format string, args ...any) error {
	return &ValidationError{Message: format, Args: args}
}

// ValidationError представляет ошибку валидации
type ValidationError struct {
	Message string
	Args    []any
}

func (e *ValidationError) Error() string {
	if len(e.Args) > 0 {
		return "validation error: " + formatMessage(e.Message, e.Args...)
	}
	return "validation error: " + e.Message
}

func formatMessage(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	// Простая замена %s на аргументы
	result := format
	for i := range args {
		placeholder := "%" + string(rune('s'+i))
		if i == 0 {
			placeholder = "%s"
		}
		result = strings.Replace(result, placeholder, "%v", 1)
	}
	return result
}
