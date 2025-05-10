package app

import (
	"context"
	"log"

	"vk-pubsub/internal/config"
	"vk-pubsub/internal/server"
)

type App struct {
	Server *server.Server
	Logger *log.Logger
	Config *config.Config
}

func NewApp(cfg *config.Config, logger *log.Logger, srv *server.Server) *App {
	return &App{
		Server: srv,
		Logger: logger,
		Config: cfg,
	}
}

func (a *App) Run() error {
	a.Logger.Printf("Запуск приложения")
	return a.Server.Start()
}

func (a *App) Stop(ctx context.Context) error {
	a.Logger.Printf("Остановка приложения")
	return a.Server.Stop(ctx)
}
