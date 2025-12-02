




package secrets

import (
	"errors"
	"os"
	"sync"
)

var (
	secretsCache     = make(map[string]string)
	userSecretsCache = make(map[string]map[string]string)
	cacheMutex       = &sync.RWMutex{}
)

func init() {
	// Load initial secrets from environment variables
	loadEnvironmentSecrets()
}

func loadEnvironmentSecrets() {
	// In a real implementation, this would load from a secure secrets manager
	// For now, we'll use environment variables as an example
	envSecrets := map[string]string{
		"llm/openai/api_key":    os.Getenv("OPENAI_API_KEY"),
		"llm/anthropic/api_key": os.Getenv("ANTHROPIC_API_KEY"),
		"llm/google/api_key":     os.Getenv("GOOGLE_API_KEY"),
		"llm/meta/api_key":      os.Getenv("META_API_KEY"),
	}

	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	for key, value := range envSecrets {
		if value != "" {
			secretsCache[key] = value
		}
	}
}

func Get(key string) (string, error) {
	cacheMutex.RLock()
	value, ok := secretsCache[key]
	cacheMutex.RUnlock()

	if !ok {
		return "", errors.New("secret not found")
	}

	return value, nil
}

func Set(key, value string) {
	cacheMutex.Lock()
	secretsCache[key] = value
	cacheMutex.Unlock()
}

func Delete(key string) {
	cacheMutex.Lock()
	delete(secretsCache, key)
	cacheMutex.Unlock()
}

func GetUserSecret(userID, key string) (string, error) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	// Check if user has their own secrets
	if userCache, ok := userSecretsCache[userID]; ok {
		if value, ok := userCache[key]; ok {
			return value, nil
		}
	}

	// Fall back to shared secrets
	value, ok := secretsCache[key]
	if !ok {
		return "", errors.New("secret not found")
	}

	return value, nil
}

func SetUserSecret(userID, key, value string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	if _, ok := userSecretsCache[userID]; !ok {
		userSecretsCache[userID] = make(map[string]string)
	}

	userSecretsCache[userID][key] = value
}

func DeleteUserSecret(userID, key string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	if userCache, ok := userSecretsCache[userID]; ok {
		delete(userCache, key)
		if len(userCache) == 0 {
			delete(userSecretsCache, userID)
		}
	}
}



