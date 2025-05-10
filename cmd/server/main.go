package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"vk-pubsub/internal/app"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load(".env")

	application, err := app.InitializeApp()
	if err != nil {
		application.Logger.Fatalf("Ошибка при инициализации приложения: %v", err)
	}

	application.Logger.Printf("Приложение инициализировано")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		application.Logger.Printf("Запуск gRPC сервера на %s", application.Config.GRPCAddress())
		if err := application.Run(); err != nil {
			application.Logger.Printf("Ошибка запуска сервера: %v", err)
			sigCh <- syscall.SIGINT
		}
	}()

	sig := <-sigCh
	application.Logger.Printf("Получен сигнал: %v, начинаем завершение работы", sig)

	ctx, cancel := context.WithTimeout(context.Background(), application.Config.ShutdownTimeout)
	defer cancel()

	if err := application.Stop(ctx); err != nil {
		application.Logger.Printf("Ошибка остановки сервера: %v", err)
		os.Exit(1)
	}

	application.Logger.Printf("Сервер остановлен")
}
