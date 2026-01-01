package http

// CreateSecretRequest представляет запрос на создание секрета
type CreateSecretRequest struct {
	Path  string `json:"path"`  // "llm/openai/api_key"
	Value string `json:"value"` // секретное значение
}

// SecretOperationResult представляет результат операции с секретом
type SecretOperationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// SecretList представляет список секретов
type SecretList struct {
	Keys []string `json:"keys"`
}
