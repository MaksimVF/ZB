# Микросервисная архитектура для сервиса Billing

## Обзор

Данная директория содержит реализацию микросервисной архитектуры для биллингового сервиса на языке Go. Архитектура разработана с учетом лучших практик микросервисов и обеспечивает высокую производительность, масштабируемость и надежность.

## Архитектура

### Компоненты

1. **Balance Service** (`balance-service`) - Управление балансом пользователей
2. **Transaction Service** (`transaction-service`) - Обработка транзакций
3. **Pricing Service** (`pricing-service`) - Управление ценами и тарифами
4. **Reservation Service** (`reservation-service`) - Система резервирования средств
5. **Payment Service** (`payment-service`) - Интеграция с платежными системами
6. **Analytics Service** (`analytics-service`) - Аналитика и отчетность
7. **API Gateway** (`gateway`) - Точка входа для всех запросов

### Связи между сервисами

```
┌─────────────┐    gRPC/HTTP     ┌──────────────────┐
│   Gateway   │◄─────────────────│ Balance Service  │
│             │                  │                  │
└─────────────┘                  └──────────────────┘
       │                                 │
       │ gRPC                            │ gRPC
       ▼                                 ▼
┌─────────────┐                  ┌──────────────────┐
│ Transaction │                  │ Pricing Service  │
│   Service   │                  │                  │
└─────────────┘                  └──────────────────┘
       │                                 │
       │ gRPC                            │ gRPC
       ▼                                 ▼
┌─────────────┐                  ┌──────────────────┐
│Reservation  │                  │ Payment Service  │
│   Service   │                  │                  │
└─────────────┘                  └──────────────────┘
                                             │
                                             │ HTTP
                                             ▼
                                    ┌──────────────────┐
                                    │Analytics Service │
                                    │                  │
                                    └──────────────────┘
```

### Хранилища данных

- **Redis** - Кэширование и временные данные
- **PostgreSQL** - Основное хранилище транзакций и аналитики
- **InfluxDB** - Метрики и временные ряды (опционально)

## Структура проекта

```
go-microservices/
├── proto/                          # gRPC протоколы
├── balance-service/                # Сервис управления балансом
├── transaction-service/            # Сервис транзакций
├── pricing-service/                # Сервис цен
├── reservation-service/            # Сервис резервирования
├── payment-service/                # Платежный сервис
├── analytics-service/              # Сервис аналитики
├── gateway/                        # API Gateway
├── common/                         # Общие компоненты
├── docker-compose.yml             # Docker Compose для всех сервисов
├── Makefile                       # Скрипты сборки и развертывания
└── README.md                      # Данный файл
```

## Быстрый старт

### Предварительные требования

- Go 1.21+
- Docker и Docker Compose
- Make
- Redis
- PostgreSQL

### Запуск всех сервисов

```bash
# Клонирование репозитория
git clone <repository>
cd services/billing/go-microservices

# Сборка всех сервисов
make build-all

# Запуск базы данных
make up-db

# Применение миграций
make migrate-up

# Запуск всех сервисов
make up-all
```

### Отдельный запуск сервисов

```bash
# Запуск сервиса баланса
make up-balance-service

# Запуск сервиса транзакций
make up-transaction-service

# Запуск API Gateway
make up-gateway
```

## Конфигурация

### Переменные окружения

Каждый сервис использует следующие переменные окружения:

- `REDIS_URL` - URL для подключения к Redis (по умолчанию: `redis://localhost:6379`)
- `POSTGRES_URL` - URL для подключения к PostgreSQL (по умолчанию: `postgres://billing:billing@localhost:5432/billing`)
- `PORT` - Порт для HTTP сервера (по умолчанию: разный для каждого сервиса)
- `GRPC_PORT` - Порт для gRPC сервера (по умолчанию: разный для каждого сервиса)
- `LOG_LEVEL` - Уровень логирования (по умолчанию: `info`)
- `JAEGER_ENDPOINT` - Endpoint для Jaeger tracing (по умолчанию: `http://localhost:14268/api/traces`)

### Настройка базы данных

```bash
# Создание базы данных
createdb billing

# Применение миграций
make migrate-up

# Откат миграций (при необходимости)
make migrate-down
```

## API Endpoints

### API Gateway (порт 8080)

- `POST /api/v1/balance/{user_id}` - Получение баланса пользователя
- `POST /api/v1/balance/{user_id}/adjust` - Корректировка баланса
- `GET /api/v1/balance/{user_id}/transactions` - История транзакций
- `POST /api/v1/reserve` - Создание резерва
- `POST /api/v1/reserve/{reservation_id}/commit` - Подтверждение резерва
- `GET /api/v1/pricing` - Получение актуальных цен
- `POST /api/v1/payment/create-checkout` - Создание платежной сессии
- `GET /api/v1/analytics/metrics` - Метрики системы

### gRPC сервисы

Каждый сервис предоставляет gRPC API на своем порту:

- **Balance Service**: gRPC порт 50051
- **Transaction Service**: gRPC порт 50052
- **Pricing Service**: gRPC порт 50053
- **Reservation Service**: gRPC порт 50054
- **Payment Service**: gRPC порт 50055
- **Analytics Service**: gRPC порт 50056

## Мониторинг и наблюдаемость

### Метрики

Каждый сервис предоставляет метрики в формате Prometheus:

- `http_requests_total` - Общее количество HTTP запросов
- `http_request_duration_seconds` - Время выполнения HTTP запросов
- `grpc_requests_total` - Общее количество gRPC запросов
- `grpc_request_duration_seconds` - Время выполнения gRPC запросов
- `balance_operations_total` - Операции с балансом
- `transaction_operations_total` - Операции с транзакциями

### Логирование

Все сервисы используют структурированное логирование в JSON формате с следующими уровнями:

- `debug` - Отладочная информация
- `info` - Общая информация
- `warn` - Предупреждения
- `error` - Ошибки

### Трассировка

Распределенная трассировка реализована с использованием Jaeger:

- Каждый запрос имеет уникальный trace_id
- Трассируются все межсервисные вызовы
- Время выполнения операций отслеживается

## Безопасность

### Аутентификация и авторизация

- JWT токены для аутентификации
- RBAC (Role-Based Access Control) для авторизации
- Шифрование чувствительных данных

### Сетевая безопасность

- Внутренняя коммуникация через gRPC с TLS
- Rate limiting на уровне API Gateway
- Input validation и sanitization

## Производительность

### Оптимизации

- Кэширование часто запрашиваемых данных в Redis
- Асинхронная обработка событий
- Batch операции для базы данных
- Connection pooling для PostgreSQL

### Масштабирование

- Горизонтальное масштабирование каждого сервиса
- Load balancing через API Gateway
- Автомасштабирование на основе метрик

## Развертывание

### Docker

Каждый сервис имеет свой Dockerfile для контейнеризации:

```bash
# Сборка образа баланс-сервиса
docker build -t balance-service:latest ./balance-service

# Запуск контейнера
docker run -d --name balance-service -p 50051:50051 balance-service:latest
```

### Kubernetes

Доступны манифесты Kubernetes для развертывания:

```bash
# Применение всех манифестов
kubectl apply -f k8s/

# Масштабирование сервиса
kubectl scale deployment balance-service --replicas=3
```

## Тестирование

### Unit тесты

```bash
# Запуск тестов для всех сервисов
make test-all

# Тесты конкретного сервиса
make test-balance-service
```

### Интеграционные тесты

```bash
# Запуск интеграционных тестов
make test-integration
```

### Нагрузочное тестирование

```bash
# Запуск нагрузочных тестов
make test-load
```

## Разработка

### Структура кода

Каждый сервис следует стандартной структуре Go проекта:

```
service-name/
├── cmd/
│   └── main.go
├── internal/
│   ├── handlers/         # HTTP/gRPC обработчики
│   ├── service/          # Бизнес-логика
│   ├── repository/       # Доступ к данным
│   ├── models/           # Модели данных
│   └── config/           # Конфигурация
├── proto/                # gRPC протоколы
├── migrations/           # Миграции БД
├── Dockerfile
├── go.mod
└── README.md
```

### Добавление нового сервиса

1. Создайте директорию для сервиса
2. Скопируйте структуру из существующего сервиса
3. Определите gRPC протоколы в `proto/`
4. Реализуйте обработчики, сервисы и репозитории
5. Добавьте тесты
6. Обновите Makefile и docker-compose.yml

## Лицензия

[Укажите лицензию проекта]

## Поддержка

Для получения поддержки или сообщения об ошибках создавайте issue в репозитории проекта.