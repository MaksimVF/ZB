# Руководство по миграции на новую архитектуру сервиса секретов

## 🚨 Внимание: Устаревшие файлы

Следующие файлы **НЕ ИСПОЛЬЗУЙТЕ** в новой разработке:

### Основные устаревшие файлы:
- ❌ `main.go.bak` - **ЗАМЕНИТЬ** на `main_new.go`
- ❌ `main_test.go.bak` - **ПЕРЕПИСАТЬ** полностью для новой архитектуры
- ❌ `main_old.go` - **УДАЛИТЬ** после проверки (резервная копия)
- ❌ `main_original.go` - **УДАЛИТЬ** после проверки (резервная копия)

### Признаки устаревших файлов:
- Комментарий `// DEPRECATED` в начале файла
- Монолитная структура без разделения на модули
- Смешение всех слоев в одном файле

## ✅ Используйте новую архитектуру

### Новые файлы и модули:
```
services/secrets-service/
├── main_new.go                    # ✅ НОВЫЙ главный файл
├── NEW_ARCHITECTURE.md           # ✅ Документация архитектуры
├── MIGRATION_GUIDE.md            # ✅ Это руководство
├── DEPRECATED.go                 # ✅ Список устаревших файлов
├── config/                       # ✅ Конфигурация
│   └── config.go
├── core/                         # ✅ Бизнес-логика
│   └── secret_service.go
├── storage/                      # ✅ Слой работы с Vault
│   └── vault.go
├── utils/                        # ✅ Утилиты
│   ├── validation.go
│   └── logger.go
├── grpc/                         # ✅ gRPC обработчики
│   ├── handler.go
│   └── logger.go
├── http/                         # ✅ HTTP админ API
│   ├── handler.go
│   ├── models.go
│   └── logger.go
└── models/                       # ✅ Структуры данных
    └── secret.go
```

## 🔄 Пошаговая миграция

### Шаг 1: Обновите импорты
```go
// Старый импорт
import (
    "context"
    // ... много других импортов в одном файле
)

// Новый импорт (чистый и понятный)
import (
    "github.com/MaksimVF/ZB/services/secrets-service/config"
    "github.com/MaksimVF/ZB/services/secrets-service/core"
    "github.com/MaksimVF/ZB/services/secrets-service/storage"
    // ... конкретные нужные импорты
)
```

### Шаг 2: Используйте новую структуру
```go
// Старый подход (неправильно)
func main() {
    // 720+ строк монолитного кода
    // Смешение всех слоев
    // Сложно тестировать
}

// Новый подход (правильно)
func main() {
    // Загрузка конфигурации
    cfg := config.Load()
    
    // Инициализация компонентов
    logger := utils.New(cfg.ServiceName)
    vaultClient := initVaultClient(cfg, logger)
    secretService := initSecretService(cfg, vaultClient, logger)
    
    // Запуск серверов
    startGRPCServer(cfg, secretService, logger)
    startHTTPServer(cfg, secretService, logger)
}
```

### Шаг 3: Обновите тесты
Старые тесты `main_test.go` **НЕ СОВМЕСТИМЫ** с новой архитектурой.

**Создайте новые тесты:**
```go
// Пример структуры нового теста
func TestSecretService(t *testing.T) {
    // Создание mock компонентов
    mockVault := NewMockVaultStorage()
    validator := &utils.Validator{}
    logger := utils.New("test")
    cfg := config.Load()
    
    // Создание сервиса
    service := core.NewSecretService(mockVault, validator, logger, cfg)
    
    // Тестирование
    // ...
}
```

## 🛠️ Разработка с новой архитектурой

### Добавление новой функции:
1. **Определите слой**: Core, Storage, Utils, HTTP, gRPC
2. **Добавьте в соответствующий модуль**
3. **Обновите интерфейсы** если нужно
4. **Добавьте тесты** для новой функции

### Пример добавления новой функции в Core:
```go
// core/secret_service.go
func (s *SecretService) NewFunction(param string) error {
    // Валидация через validator
    if err := s.validator.ValidateParam(param); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    
    // Логирование
    s.logger.Info().Str("param", param).Msg("Processing new function")
    
    // Бизнес-логика
    // ...
    
    return nil
}
```

### Пример добавления нового HTTP эндпоинта:
```go
// http/handler.go
func (h *Handler) NewEndpoint(w http.ResponseWriter, r *http.Request) {
    // Обработка запроса
    // ...
    h.sendSuccess(w, r, result)
}
```

## ⚠️ Важные моменты

### 1. НЕ ИСПОЛЬЗУЙТЕ устаревшие файлы
- Все файлы с `// DEPRECATED` предназначены только для справки
- Новый код должен использовать только новую архитектуру

### 2. Сохраняйте разделение ответственности
- **Core** - только бизнес-логика
- **Storage** - только работа с Vault
- **HTTP/gRPC** - только обработка запросов
- **Utils** - только переиспользуемые компоненты

### 3. Следуйте паттернам
- Используйте существующие паттерны логирования
- Следуйте паттернам валидации
- Используйте структурированные ошибки

## 🔍 Проверка правильности

### Перед коммитом убедитесь:
- [ ] Не используете файлы с `// DEPRECATED`
- [ ] Новый код находится в правильном модуле
- [ ] Следуете паттернам архитектуры
- [ ] Добавили тесты для новой функциональности
- [ ] Обновили документацию если нужно

## 📞 Получение помощи

При возникновении вопросов:
1. Изучите `NEW_ARCHITECTURE.md`
2. Посмотрите примеры в существующих модулях
3. Обратитесь к команде разработки

## 🎯 Цель миграции

После полной миграции у нас будет:
- ✅ Чистый, понятный код
- ✅ Легкость поддержки и развития
- ✅ Простота тестирования
- ✅ Возможность замены компонентов
- ✅ Масштабируемость архитектуры