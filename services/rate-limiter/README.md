# Rate Limiter Service

Высокопроизводительный сервис ограничения скорости запросов для архитектуры ZB, обеспечивающий контроль доступа к API на основе различных типов аутентификации.

## 🚀 Возможности

### Основная функциональность
- **Контроль скорости запросов** по типам аутентификации (JWT, API Key, Anonymous)
- **Redis-персистентность** для надежного хранения данных
- **gRPC API** для высокопроизводительного межсервисного взаимодействия
- **Prometheus метрики** для мониторинга производительности
- **TLS/mTLS безопасность** для защищенной коммуникации
- **Автоматическая инициализация лимитов** для различных endpoints

### Алгоритмы ограничения
- **Sliding Window** для контроля количества запросов
- **Token Bucket** для контроля потребления ресурсов
- **Graceful degradation** при сбоях Redis

### Безопасность
- **mTLS** для gRPC коммуникации
- **Fallback** на insecure режим при отсутствии сертификатов
- **Валидация входных параметров** для предотвращения атак

## 🏗️ Архитектура

### Компоненты системы
1. **gRPC Server**: Основной сервер для обработки запросов ограничения скорости
2. **Redis Integration**: Персистентное хранение данных о использовании
3. **Metrics Server**: HTTP сервер для Prometheus метрик
4. **Configuration Manager**: Управление лимитами и конфигурацией

### Endpoints и лимиты
По умолчанию сервис поддерживает следующие endpoints:

- `/v1/chat/completions` - 60 JWT / 30 API Key / 5 Anonymous запросов в минуту
- `/v1/completions` - 60 JWT / 30 API Key / 5 Anonymous запросов в минуту  
- `/v1/embeddings` - 120 JWT / 60 API Key / 10 Anonymous запросов в минуту
- `/v1/agentic` - 30 JWT / 15 API Key / 3 Anonymous запроса в минуту

## 🔌 API

### gRPC Методы

#### Check
Проверяет, разрешен ли запрос в рамках лимитов скорости.

```protobuf
rpc Check(CheckRequest) returns (CheckResponse)
```

**CheckRequest:**
- `authorization` (string): JWT токен или API ключ
- `path` (string): Путь к endpoint'у

**CheckResponse:**
- `allowed` (bool): Разрешен ли запрос
- `retry_after_secs` (uint32): Время ожидания в секундах при превышении лимита

#### SetLimit
Устанавливает лимит скорости для определенного path и типа аутентификации.

```protobuf
rpc SetLimit(SetLimitRequest) returns (SetLimitResponse)
```

**SetLimitRequest:**
- `path` (string): Путь к endpoint'у
- `auth_type` (string): Тип аутентификации ("jwt", "api_key", "anonymous")
- `limit` (int32): Количество запросов в минуту

#### GetLimits
Получает все текущие лимиты.

```protobuf
rpc GetLimits(GetLimitsRequest) returns (GetLimitsResponse)
```

## 📊 Мониторинг

### Prometheus Метрики
Сервис предоставляет следующие метрики:

- `rate_limiter_check_requests_total`: Общее количество проверок
- `rate_limiter_check_allowed_total`: Количество разрешенных запросов
- `rate_limiter_check_denied_total`: Количество отклоненных запросов
- `rate_limiter_set_limit_requests_total`: Количество запросов на установку лимитов
- `rate_limiter_get_limit_requests_total`: Количество запросов на получение лимитов
- `rate_limiter_active_requests`: Текущее количество активных запросов

### Метрики Endpoint
Метрики доступны на `http://localhost:8086/metrics`

## 🐳 Развертывание

### Docker
```bash
# Сборка образа
docker build -t rate-limiter .

# Запуск контейнера
docker run -d \
  --name rate-limiter \
  -p 50051:50051 \
  -p 8086:8086 \
  --network=zb_network \
  rate-limiter
```

### Docker Compose
```yaml
version: '3.8'
services:
  rate-limiter:
    build: ./services/rate-limiter
    ports:
      - "50051:50051"
      - "8086:8086"
    volumes:
      - ./certs:/certs:ro
    environment:
      - REDIS_URL=redis:6379
    depends_on:
      - redis
    networks:
      - zb_network
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    networks:
      - zb_network
```

### Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rate-limiter
spec:
  replicas: 3
  selector:
    matchLabels:
      app: rate-limiter
  template:
    metadata:
      labels:
        app: rate-limiter
    spec:
      containers:
      - name: rate-limiter
        image: rate-limiter:latest
        ports:
        - containerPort: 50051
        - containerPort: 8086
        env:
        - name: REDIS_URL
          value: "redis:6379"
        volumeMounts:
        - name: certs
          mountPath: /certs
          readOnly: true
      volumes:
      - name: certs
        secret:
          secretName: rate-limiter-certs
---
apiVersion: v1
kind: Service
metadata:
  name: rate-limiter
spec:
  selector:
    app: rate-limiter
  ports:
  - name: grpc
    port: 50051
    targetPort: 50051
  - name: metrics
    port: 8086
    targetPort: 8086
```

## ⚙️ Конфигурация

### Переменные окружения
- `REDIS_URL`: URL Redis сервера (по умолчанию: `redis:6379`)
- `TLS_CERT_PATH`: Путь к TLS сертификату (по умолчанию: `/certs/rate-limiter.pem`)
- `TLS_KEY_PATH`: Путь к TLS ключу (по умолчанию: `/certs/rate-limiter-key.pem`)

### TLS Сертификаты
Сертификаты должны быть расположены в `/certs/`:
- `rate-limiter.pem` - сертификат сервера
- `rate-limiter-key.pem` - приватный ключ

При отсутствии сертификатов сервис автоматически переключается на insecure режим.

## 🧪 Тестирование

### Unit Tests
```bash
go test ./...
```

### Integration Tests
```bash
go test -tags integration ./...
```

### Performance Tests
```bash
go test -bench=. ./...
```

### Manual Testing
```bash
# Проверка статуса
grpcurl -plaintext localhost:50051 rate_limiter.RateLimiter/GetLimits

# Установка лимита
grpcurl -plaintext -d '{"path":"/test","auth_type":"jwt","limit":10}' \
  localhost:50051 rate_limiter.RateLimiter/SetLimit

# Проверка лимита
grpcurl -plaintext -d '{"authorization":"Bearer test","path":"/test"}' \
  localhost:50051 rate_limiter.RateLimiter/Check
```

## 📈 Производительность

### Оптимизации
- **Connection pooling** для Redis
- **Pipeline операции** для атомарных обновлений
- **In-memory fallback** при сбоях Redis
- **Efficient data structures** для быстрого доступа к данным

### Рекомендации
- Мониторить latency gRPC запросов (цель: <5ms)
- Отслеживать hit rate Redis (цель: >95%)
- Наблюдать за количеством отклоненных запросов
- Анализировать паттерны использования API

### Масштабирование
- Сервис stateless и может масштабироваться горизонтально
- Redis должен быть настроен с кластеризацией для высокой нагрузки
- Рекомендуется использовать load balancer для gRPC трафика

## 🔧 Разработка

### Требования
- Go 1.25+
- Redis 7+
- protoc 3.x

### Локальная разработка
```bash
# Клонирование и сборка
cd services/rate-limiter
go mod tidy
go build -o rate-limiter .

# Запуск тестов
go test -v ./...

# Генерация proto файлов
protoc --go_out=. --go-grpc_out=. rate_limiter.proto
```

### Структура проекта
```
├── internal/
│   └── server/
│       ├── server.go          # Основная логика сервера
│       └── server_test.go     # Unit тесты
├── pb/                        # Сгенерированные proto файлы
├── rate_limiter.proto         # Protocol Buffers определения
├── main.go                    # Точка входа
├── go.mod                     # Зависимости Go
├── go.sum                     # Checksums зависимостей
├── Dockerfile                 # Docker конфигурация
└── README.md                  # Документация
```

## 🛠️ Устранение неполадок

### Частые проблемы

#### Redis недоступен
```
Warning: Failed to connect to Redis: connect timeout
```
**Решение**: Проверить доступность Redis, настройки сети и переменную REDIS_URL

#### TLS сертификаты не найдены
```
Failed to load TLS credentials: open /certs/rate-limiter.pem: no such file
```
**Решение**: Сертификаты будут созданы автоматически или помещены в /certs/

#### Высокая latency
**Решение**: Проверить производительность Redis, настроить connection pooling

### Логи
Сервис выводит логи в stdout в структурированном формате:
```
2025-12-28T13:18:19Z INFO Rate limiter service running on :50051 (TLS enabled)
2025-12-28T13:18:19Z INFO Metrics server starting on :8086
```

### Health Checks
```bash
# Проверка health status
curl http://localhost:8086/metrics | grep rate_limiter

# Проверка Redis подключения
redis-cli -h redis -p 6379 ping
```

## 🤝 Участие в разработке

1. Создайте feature branch
2. Внесите изменения с соответствующими тестами
3. Убедитесь что все тесты проходят (`go test ./...`)
4. Проверьте код на соответствие стандартам (`go vet ./...`)
5. Обновите документацию при необходимости
6. Создайте Pull Request

## 📄 Лицензия

Этот проект является частью архитектуры ZB и используется для внутренних целей.

---

**Rate Limiter Service** - Надежный контроль доступа для высоконагруженных API 🚀