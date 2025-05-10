package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"vk-pubsub/pkg/proto"
)

var (
	addr       = flag.String("addr", "localhost:50052", "Адрес сервера")
	action     = flag.String("action", "subscribe", "Действие: subscribe или publish")
	key        = flag.String("key", "test-key", "Ключ для подписки или публикации")
	message    = flag.String("message", "Hello, World!", "Сообщение для публикации")
	publishNum = flag.Int("count", 1, "Количество сообщений для публикации")
)

func main() {
	flag.Parse()

	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Не удалось подключиться: %v", err)
	}
	defer conn.Close()

	client := proto.NewPubSubClient(conn)

	switch *action {
	case "subscribe":
		subscribe(client, *key)
	case "publish":
		publish(client, *key, *message, *publishNum)
	default:
		log.Fatalf("Неизвестное действие: %s. Используйте 'subscribe' или 'publish'", *action)
	}
}

func subscribe(client proto.PubSubClient, key string) {
	fmt.Printf("Подписка на ключ: %s\n", key)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.Subscribe(ctx, &proto.SubscribeRequest{
		Key: key,
	})
	if err != nil {
		log.Fatalf("Ошибка при подписке: %v", err)
	}

	fmt.Println("Подписка установлена. Ожидание сообщений...")
	fmt.Println("Нажмите Ctrl+C для выхода")

	for {
		event, err := stream.Recv()
		if err == io.EOF {
			fmt.Println("Стрим закрыт сервером")
			break
		}
		if err != nil {
			log.Fatalf("Ошибка при получении события: %v", err)
		}

		fmt.Printf("Получено событие: %s\n", event.Data)
	}
}

func publish(client proto.PubSubClient, key, message string, count int) {
	fmt.Printf("Публикация %d сообщений для ключа: %s\n", count, key)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	for i := 0; i < count; i++ {
		msg := message
		if count > 1 {
			msg = fmt.Sprintf("%s (%d)", message, i+1)
		}

		_, err := client.Publish(ctx, &proto.PublishRequest{
			Key:  key,
			Data: msg,
		})
		if err != nil {
			log.Fatalf("Ошибка при публикации: %v", err)
		}

		fmt.Printf("Сообщение опубликовано: %s\n", msg)

	}

	fmt.Println("Публикация завершена")
}
