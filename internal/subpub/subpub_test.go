package subpub

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSubscribeAndPublish(t *testing.T) {
	sp := NewSubPub()
	var wg sync.WaitGroup
	received := make(chan string, 1)

	// Подписываемся на событие
	sub, err := sp.Subscribe("test", func(msg interface{}) {
		if str, ok := msg.(string); ok {
			received <- str
		}
		wg.Done()
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Публикуем сообщение
	wg.Add(1)
	err = sp.Publish("test", "hello")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Ждем завершения обработки сообщения
	wg.Wait()

	// Проверяем получение сообщения
	select {
	case msg := <-received:
		if msg != "hello" {
			t.Errorf("Expected 'hello', got '%s'", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for message")
	}

	// Отписываемся
	sub.Unsubscribe()

	// Проверяем, что после отписки сообщения не приходят
	err = sp.Publish("test", "world")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	select {
	case msg := <-received:
		t.Errorf("Unexpected message received after unsubscribe: %s", msg)
	case <-time.After(time.Second):
		// Ожидаемое поведение - сообщение не должно прийти
	}
}

func TestClose(t *testing.T) {
	sp := NewSubPub()
	ctx := context.Background()

	// Подписываемся на событие
	_, err := sp.Subscribe("test", func(msg interface{}) {})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Закрываем систему
	err = sp.Close(ctx)
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Проверяем, что после закрытия нельзя подписаться
	_, err = sp.Subscribe("test", func(msg interface{}) {})
	if err != ErrClosed {
		t.Errorf("Expected ErrClosed, got %v", err)
	}

	// Проверяем, что после закрытия нельзя публиковать
	err = sp.Publish("test", "hello")
	if err != ErrClosed {
		t.Errorf("Expected ErrClosed, got %v", err)
	}
}

func TestMultipleSubscribers(t *testing.T) {
	sp := NewSubPub()
	var wg sync.WaitGroup
	received := make(chan string, 2)

	// Создаем двух подписчиков
	for i := 0; i < 2; i++ {
		wg.Add(1)
		_, err := sp.Subscribe("test", func(msg interface{}) {
			if str, ok := msg.(string); ok {
				received <- str
			}
			wg.Done()
		})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
	}

	// Публикуем сообщение
	err := sp.Publish("test", "hello")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Ждем получения сообщений обоими подписчиками
	timeout := time.After(time.Second)
	for i := 0; i < 2; i++ {
		select {
		case msg := <-received:
			if msg != "hello" {
				t.Errorf("Expected 'hello', got '%s'", msg)
			}
		case <-timeout:
			t.Fatal("Timeout waiting for message")
		}
	}
}

func TestCloseWithContext(t *testing.T) {
	sp := NewSubPub()
	var wg sync.WaitGroup
	handlerDone := make(chan struct{})

	// Создаем много подписчиков, чтобы закрытие заняло время
	for i := 0; i < 1000; i++ {
		_, err := sp.Subscribe(fmt.Sprintf("test%d", i), func(msg interface{}) {})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
	}

	// Создаем подписчика с долгим хендлером
	_, err := sp.Subscribe("test", func(msg interface{}) {
		wg.Add(1)
		defer wg.Done()
		// Имитируем долгую работу
		time.Sleep(100 * time.Millisecond)
		close(handlerDone)
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Публикуем сообщение
	err = sp.Publish("test", "test")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Даем хендлеру время начать работу
	time.Sleep(50 * time.Millisecond)

	// Создаем контекст с очень маленьким таймаутом и сразу отменяем его тоже,
	// чтобы тест был более надежным
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Дополнительно отменяем контекст, чтобы гарантировать его отмену
	cancel()

	// Закрываем систему
	err = sp.Close(ctx)
	if err == nil || (err != context.DeadlineExceeded && err != context.Canceled) {
		t.Errorf("Expected context.DeadlineExceeded or context.Canceled, got %v", err)
	}

	// Проверяем, что система закрыта
	_, err = sp.Subscribe("test", func(msg interface{}) {})
	if err != ErrClosed {
		t.Errorf("Expected ErrClosed, got %v", err)
	}

	// Ждем завершения хендлера
	select {
	case <-handlerDone:
		// Хендлер должен завершиться
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Handler did not complete")
	}
}

func TestCloseWithCancelledContext(t *testing.T) {
	sp := NewSubPub()
	var wg sync.WaitGroup
	handlerDone := make(chan struct{})

	// Создаем подписчика с долгим хендлером
	_, err := sp.Subscribe("test", func(msg interface{}) {
		wg.Add(1)
		defer wg.Done()
		// Имитируем долгую работу
		time.Sleep(200 * time.Millisecond)
		close(handlerDone)
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Публикуем сообщение
	err = sp.Publish("test", "test")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Создаем контекст и сразу отменяем его
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Закрываем систему
	err = sp.Close(ctx)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	// Проверяем, что система закрыта
	_, err = sp.Subscribe("test", func(msg interface{}) {})
	if err != ErrClosed {
		t.Errorf("Expected ErrClosed, got %v", err)
	}

	// Ждем завершения хендлера
	select {
	case <-handlerDone:
		// Хендлер должен завершиться
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Handler did not complete")
	}
}

func TestFIFOOrder(t *testing.T) {
	sp := NewSubPub()
	var received []int
	var mu sync.Mutex

	// Создаем подписчика
	_, err := sp.Subscribe("test", func(msg interface{}) {
		if num, ok := msg.(int); ok {
			mu.Lock()
			received = append(received, num)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Публикуем сообщения в порядке
	for i := 0; i < 1000; i++ {
		err := sp.Publish("test", i)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
	}

	// Ждем обработки всех сообщений
	time.Sleep(100 * time.Millisecond)

	// Проверяем порядок
	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1000 {
		t.Errorf("Expected 1000 messages, got %d", len(received))
	}
	for i := 0; i < len(received); i++ {
		if received[i] != i {
			t.Errorf("Expected message %d, got %d", i, received[i])
		}
	}
}

func TestMessageProcessingOnClose(t *testing.T) {
	sp := NewSubPub()
	var received []int
	var mu sync.Mutex

	// Создаем подписчика с медленной обработкой
	_, err := sp.Subscribe("test", func(msg interface{}) {
		if num, ok := msg.(int); ok {
			time.Sleep(10 * time.Millisecond) // Имитируем медленную обработку
			mu.Lock()
			received = append(received, num)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Публикуем сообщения
	for i := 0; i < 100; i++ {
		err := sp.Publish("test", i)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
	}

	// Даем время на начало обработки
	time.Sleep(50 * time.Millisecond)

	// Закрываем систему
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = sp.Close(ctx)
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Проверяем, что все сообщения были обработаны
	mu.Lock()
	if len(received) != 100 {
		t.Errorf("Expected 100 messages, got %d", len(received))
	}
	for i := 0; i < len(received); i++ {
		if received[i] != i {
			t.Errorf("Expected message %d, got %d", i, received[i])
		}
	}
	mu.Unlock()
}

func TestUnsubscribeWithPendingMessages(t *testing.T) {
	sp := NewSubPub()
	var received []int
	var mu sync.Mutex

	// Создаем подписчика
	sub, err := sp.Subscribe("test", func(msg interface{}) {
		if num, ok := msg.(int); ok {
			time.Sleep(10 * time.Millisecond) // Имитируем медленную обработку
			mu.Lock()
			received = append(received, num)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Публикуем сообщения
	for i := 0; i < 100; i++ {
		err := sp.Publish("test", i)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
	}

	// Даем время на начало обработки
	time.Sleep(50 * time.Millisecond)

	// Отписываемся
	sub.Unsubscribe()

	// Ждем завершения обработки
	time.Sleep(200 * time.Millisecond)

	// Проверяем, что все сообщения были обработаны
	mu.Lock()
	if len(received) != 100 {
		t.Errorf("Expected 100 messages, got %d", len(received))
	}
	for i := 0; i < len(received); i++ {
		if received[i] != i {
			t.Errorf("Expected message %d, got %d", i, received[i])
		}
	}
	mu.Unlock()
}
