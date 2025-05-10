package app

import (
	"log"
	"os"

	"github.com/google/wire"

	"vk-pubsub/internal/config"
	"vk-pubsub/internal/server"
	"vk-pubsub/internal/subpub"
)

// ProvideLogger создает и возвращает настроенный логгер
func ProvideLogger() *log.Logger {
	return log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
}

// ProvideConfig создает и возвращает конфигурацию
func ProvideConfig() *config.Config {
	return config.NewConfig()
}

// ProvideSubPub создает и возвращает систему публикации-подписки
func ProvideSubPub(logger *log.Logger) subpub.SubPub {
	return subpub.NewSubPubWithLogger(logger)
}

// ProvideServer создает и возвращает сервер
func ProvideServer(cfg *config.Config, logger *log.Logger, pubSub subpub.SubPub) *server.Server {
	return server.New(cfg, logger, pubSub)
}

// AppSet предоставляет набор провайдеров для создания приложения
var AppSet = wire.NewSet(
	ProvideLogger,
	ProvideConfig,
	ProvideSubPub,
	ProvideServer,
	NewApp,
)
