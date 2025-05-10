# Сервис подписок PubSub

Сервис подписок, работающий по gRPC. Позволяет публиковать события по ключу и подписываться на события по ключу.

## Описание проекта

Проект состоит из трех основных частей:
1. Пакет `subpub` - реализация системы публикации-подписки, работающая в памяти
2. gRPC сервис - реализация gRPC API для взаимодействия с системой публикации-подписки
3. Пакет `app` - реализация Dependency Injection с использованием Google Wire

## Сборка и запуск

### Зависимости

- Go 1.23+
- protoc (для генерации gRPC кода)
- Google Wire (для инъекции зависимостей)

### Установка зависимостей

```bash
go mod tidy
go get github.com/google/wire/cmd/wire
```

### Сборка

```bash
make build
# или
go build -o build/pubsub-server ./cmd/server
```

### Запуск сервера

```bash
make run
# или
./build/pubsub-server
# или
go run ./cmd/server/main.go
```

## Тестирование

Проект включает набор тестов для подсистемы публикации-подписки:

```bash
make test
# или
go test -v ./internal/subpub
```

Тесты subpub проверяют:
- Корректную публикацию и доставку сообщений
- Отписку подписчиков
- Обработку конкурентных подписок и публикаций
- Корректное завершение работы системы
- Обработку граничных случаев

## Использование клиента

Для отправки и получения сообщений используйте клиент из папки `cmd/client`:

### Подписка на события

```bash
go run cmd/client/main.go --action=subscribe --key=test-key
```

### Публикация событий

```bash
go run cmd/client/main.go --action=publish --key=test-key --message="Ваше сообщение" --count=5
```

## API Описание

Сервис предоставляет два метода:

### Subscribe

Позволяет подписаться на поток событий по ключу.

```protobuf
rpc Subscribe(SubscribeRequest) returns (stream Event);

message SubscribeRequest {
    string key = 1;
}

message Event {
    string data = 1;
}
```

### Publish

Позволяет опубликовать событие по ключу.

```protobuf
rpc Publish(PublishRequest) returns (google.protobuf.Empty);

message PublishRequest {
    string key = 1;
    string data = 2;
}
```

## Структура проекта

```
.
├── cmd/                  # Исполняемые приложения
│   ├── server/           # gRPC сервер
│   └── client/           # gRPC клиент
├── internal/             # Внутренний код приложений
│   ├── app/              # DI с помощью Wire
│   ├── config/           # Конфигурация
│   ├── server/           # Логика gRPC сервера
│   └── subpub/           # Система публикации-подписки
├── pkg/                  # Код для внешнего использования
│   └── proto/            # Protobuf-определения
├── build/                # Директория для собранных бинарных файлов
└── Makefile              # Автоматизация сборки
```

## Использованные паттерны
- **Dependency Injection** - внедрение зависимостей с помощью Google Wire
- **Repository Pattern** - абстракция для подписок и публикаций
- **Graceful Shutdown** - корректное завершение работы сервиса
- **Clean Architecture** - соблюдение принципа независимости бизнес-логики от внешних интерфейсов, реализовано через изоляцию основной логики в internal от внешних компонентов в pkg