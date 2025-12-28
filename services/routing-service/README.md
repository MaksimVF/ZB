# Routing Service

Высокопроизводительный сервис маршрутизации для архитектуры ZB, обеспечивающий динамическое управление головными узлами и интеллектуальную маршрутизацию запросов.

## 🚀 Возможности

### Основная функциональность
- **Динамическая регистрация головных узлов** с мониторингом состояния в реальном времени
- **Интеллектуальная маршрутизация** на основе множества стратегий (round-robin, least-loaded, geo-preferred, model-specific, adaptive, predictive)
- **Управление политиками маршрутизации** через REST API
- **Высокопроизводительный gRPC API** для межсервисного взаимодействия
- **REST API** для администрирования и мониторинга
- **Redis-хранилище** для персистентности данных
- **NATS messaging** для асинхронных событий

### Протоколы и интерфейсы
- **gRPC** с mTLS безопасностью
- **HTTP/REST** с JWT аутентификацией
- **WebSocket** для двунаправленной связи
- **Server-Sent Events (SSE)** для обновлений в реальном времени
- **Webhook endpoints** для внешних интеграций

### Безопасность
- **mTLS** для gRPC коммуникации
- **JWT аутентификация** для HTTP API
- **RBAC** (Role-Based Access Control)
- **Подпись приложений** для webhook'ов
- **Rate limiting** для защиты от злоупотреблений

### Мониторинг и наблюдаемость
- **Prometheus метрики** для сбора данных
- **Grafana dashboards** для визуализации
- **Структурированное логирование** с Zap
- **Audit trails** для критических операций
- **Health checks** для мониторинга состояния

## 🏗️ Архитектура

### Компоненты системы
1. **gRPC Server**: Высокопроизводительное межсервисное взаимодействие
2. **HTTP Server**: REST API для администрирования и мониторинга
3. **Redis Backend**: Хранилище для регистраций головных узлов и политик
4. **NATS Integration**: Асинхронная обработка событий
5. **Routing Engine**: Интеллектуальный движок принятия решений
6. **Cache Layer**: Кэширование решений маршрутизации

### Стратегии маршрутизации
- **Round Robin**: Равномерное распределение нагрузки
- **Least Loaded**: Выбор узла с минимальной нагрузкой
- **Geo Preferred**: Предпочтение узлов в определенном регионе
- **Model Specific**: Маршрутизация на основе типа модели
- **Adaptive**: Адаптивная маршрутизация с учетом множества факторов
- **Predictive**: Предсказательная балансировка нагрузки
- **Hybrid**: Комбинация нескольких стратегий

## 🔌 API

### gRPC Методы

```protobuf
service RoutingService {
  rpc RegisterHead(RegisterHeadRequest) returns (RegisterHeadResponse);
  rpc UpdateHeadStatus(UpdateHeadStatusRequest) returns (UpdateHeadStatusResponse);
  rpc GetRoutingDecision(GetRoutingDecisionRequest) returns (GetRoutingDecisionResponse);
  rpc GetAllHeads(GetAllHeadsRequest) returns (GetAllHeadsResponse);
  rpc UpdateRoutingPolicy(UpdateRoutingPolicyRequest) returns (UpdateRoutingPolicyResponse);
  rpc GetRoutingPolicy(GetRoutingPolicyRequest) returns (GetRoutingPolicyResponse);
}
```

### REST Endpoints

#### Администрирование
- `GET /api/routing/policy` - Получить текущую политику маршрутизации
- `PUT /api/routing/policy` - Обновить политику маршрутизации (требуется роль Admin)
- `GET /api/routing/heads` - Получить все головные узлы
- `POST /api/routing/heads` - Зарегистрировать новый головной узел (требуется роль Operator)

#### Webhook endpoints
- `POST /webhook/head-status` - Обновление статуса головного узла
- `POST /webhook/routing-decision` - Запрос решения маршрутизации

#### Реальное время
- `GET /events/head-status` - Server-Sent Events для статуса узлов
- `GET /events/routing-decisions` - SSE для решений маршрутизации
- `GET /ws/head-management` - WebSocket для управления узлами
- `GET /ws/routing-decisions` - WebSocket для решений маршрутизации

#### Система
- `GET /health` - Проверка здоровья системы
- `GET /metrics` - Prometheus метрики

## ⚙️ Конфигурация

### Переменные окружения
- `REDIS_URL` - URL Redis сервера
- `NATS_URL` - URL NATS сервера
- `HTTP_PORT` - Порт HTTP сервера (по умолчанию: :8080)
- `GRPC_PORT` - Порт gRPC сервера (по умолчанию: :50061)
- `JWT_SECRET` - Секретный ключ для JWT токенов

### Токены аутентификации
- `Bearer admin-token` - Полный доступ (Admin)
- `Bearer operator-token` - Операционный доступ (Operator)
- `Bearer viewer-token` - Только чтение (Viewer)
- `Bearer webhook-token` - Доступ к webhook'ам

## 🐳 Развертывание

### Docker
```bash
# Сборка образа
docker build -t routing-service .

# Запуск контейнера
docker run -d \
  --name routing-service \
  -p 8080:8080 \
  -p 50061:50061 \
  --network=zb_network \
  routing-service
```

### Docker Compose
```yaml
version: '3.8'
services:
  routing-service:
    build: .
    ports:
      - "8080:8080"
      - "50061:50061"
    environment:
      - REDIS_URL=redis:6379
      - NATS_URL=nats:4222
    depends_on:
      - redis
      - nats
    networks:
      - zb_network

  redis:
    image: redis:7-alpine
    networks:
      - zb_network

  nats:
    image: nats:2-alpine
    networks:
      - zb_network
```

## 📊 Мониторинг

### Метрики
- `routing_decisions_total` - Общее количество решений маршрутизации
- `head_registrations_total` - Количество регистраций головных узлов
- `active_heads` - Количество активных узлов
- `cache_hits_total` / `cache_misses_total` - Статистика кэша
- `http_requests_total` - HTTP запросы
- `websocket_connections` - Активные WebSocket соединения

### Grafana Dashboards
Доступны две преднастроенные панели:
- **Basic Dashboard**: Основные метрики маршрутизации
- **Enhanced Dashboard**: Расширенная аналитика с latency и системными метриками

### Alerts
- Высокий уровень ошибок маршрутизации
- Малое количество активных узлов
- Высокая HTTP latency
- Высокое использование ресурсов

## 🔒 Безопасность

### Сертификаты
Для работы mTLS необходимы сертификаты:
```bash
cd certs
./generate_certs.sh
```

Это создаст:
- CA сертификат и ключ
- Серверный сертификат и ключ
- Клиентский сертификат и ключ

### Аутентификация и авторизация
- JWT токены для HTTP API
- mTLS сертификаты для gRPC
- RBAC с тремя уровнями доступа
- Подпись приложений для webhook'ов

## 🧪 Тестирование

### Unit Tests
```bash
go test ./...
```

### Интеграционные тесты
```bash
go test -tags integration ./...
```

### Performance Tests
```bash
go test -tags perf ./...
```

## 📈 Производительность

### Оптимизации
- **Кэширование решений** маршрутизации для частых запросов
- **Connection pooling** для Redis и NATS
- **Batch операции** для снижения сетевых накладных расходов
- **Предсказательная балансировка** нагрузки
- **Circuit breakers** для защиты от внешних сервисов

### Рекомендации
- Мониторить cache hit rate (цель: >90%)
- Отслеживать latency маршрутизации (цель: <10ms)
- Наблюдать за количеством активных соединений
- Анализировать эффективность стратегий маршрутизации

## 🛠️ Разработка

### Требования
- Go 1.25+
- Redis 7+
- NATS 2+

### Локальная разработка
```bash
# Клонирование и сборка
git clone <repository>
cd services/routing-service
go mod tidy
go build -o routing-service .

# Запуск тестов
go test -v ./...

# Генерация сертификатов
cd certs && ./generate_certs.sh
```

### Структура проекта
```
├── grpc/           # gRPC обработчики
├── http/           # HTTP API и middleware
├── routing/        # Движок маршрутизации и стратегии
├── storage/        # Redis интеграция
├── monitoring/     # Метрики и мониторинг
├── middleware/     # Аудит и дополнительные middleware
├── config/         # Конфигурация
├── proto/          # Protocol Buffers определения
├── certs/          # TLS сертификаты
├── grafana/        # Grafana dashboards
├── prometheus/     # Prometheus конфигурация
└── test-runner/    # Инструменты тестирования
```

## 📝 Документация

### Дополнительные документы
- **[IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md)** - План реализации и статус выполнения
- **[MONITORING.md](MONITORING.md)** - Подробное описание мониторинга и метрик
- **[PERFORMANCE.md](PERFORMANCE.md)** - Оптимизация производительности
- **[SECURITY.md](SECURITY.md)** - Реализация безопасности

## 🤝 Участие в разработке

1. Создайте feature branch
2. Внесите изменения с соответствующими тестами
3. Убедитесь что все тесты проходят
4. Обновите документацию при необходимости
5. Создайте Pull Request

## 📄 Лицензия

Этот проект является частью архитектуры ZB и используется для внутренних целей.

---

**Routing Service** - Интеллектуальная маршрутизация для современных микросервисных архитектур 🚀
