package subpub

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestSubscribeAndPublish(t *testing.T) {
	sp := NewSubPub()
	var wg sync.WaitGroup
	received := make(chan string, 1)

	sub, err := sp.Subscribe("test", func(msg interface{}) {
		if str, ok := msg.(string); ok {
			received <- str
		}
		wg.Done()
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	wg.Add(1)
	err = sp.Publish("test", "hello")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	wg.Wait()

	select {
	case msg := <-received:
		if msg != "hello" {
			t.Errorf("Expected 'hello', got '%s'", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for message")
	}

	sub.Unsubscribe()

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

	_, err := sp.Subscribe("test", func(msg interface{}) {})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	err = sp.Close(ctx)
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	_, err = sp.Subscribe("test", func(msg interface{}) {})
	if err != ErrClosed {
		t.Errorf("Expected ErrClosed, got %v", err)
	}

	err = sp.Publish("test", "hello")
	if err != ErrClosed {
		t.Errorf("Expected ErrClosed, got %v", err)
	}
}

func TestMultipleSubscribers(t *testing.T) {
	sp := NewSubPub()
	var wg sync.WaitGroup
	received := make(chan string, 2)

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

	err := sp.Publish("test", "hello")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

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

	for i := 0; i < 1000; i++ {
		_, err := sp.Subscribe(fmt.Sprintf("test%d", i), func(msg interface{}) {})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
	}

	_, err := sp.Subscribe("test", func(msg interface{}) {
		wg.Add(1)
		defer wg.Done()
		time.Sleep(100 * time.Millisecond)
		close(handlerDone)
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	err = sp.Publish("test", "test")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	cancel()

	err = sp.Close(ctx)
	if err == nil || (err != context.DeadlineExceeded && err != context.Canceled) {
		t.Errorf("Expected context.DeadlineExceeded or context.Canceled, got %v", err)
	}

	_, err = sp.Subscribe("test", func(msg interface{}) {})
	if err != ErrClosed {
		t.Errorf("Expected ErrClosed, got %v", err)
	}

	select {
	case <-handlerDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Handler did not complete")
	}
}

func TestCloseWithCancelledContext(t *testing.T) {
	sp := NewSubPub()
	var wg sync.WaitGroup
	handlerDone := make(chan struct{})

	_, err := sp.Subscribe("test", func(msg interface{}) {
		wg.Add(1)
		defer wg.Done()
		time.Sleep(200 * time.Millisecond)
		close(handlerDone)
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	err = sp.Publish("test", "test")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = sp.Close(ctx)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	_, err = sp.Subscribe("test", func(msg interface{}) {})
	if err != ErrClosed {
		t.Errorf("Expected ErrClosed, got %v", err)
	}

	select {
	case <-handlerDone:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Handler did not complete")
	}
}

func TestFIFOOrder(t *testing.T) {
	sp := NewSubPub()
	var received []int
	var mu sync.Mutex

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

	for i := 0; i < 1000; i++ {
		err := sp.Publish("test", i)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond)

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

	_, err := sp.Subscribe("test", func(msg interface{}) {
		if num, ok := msg.(int); ok {
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			received = append(received, num)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	for i := 0; i < 100; i++ {
		err := sp.Publish("test", i)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
	}

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = sp.Close(ctx)
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

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

	sub, err := sp.Subscribe("test", func(msg interface{}) {
		if num, ok := msg.(int); ok {
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			received = append(received, num)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	for i := 0; i < 100; i++ {
		err := sp.Publish("test", i)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
	}

	time.Sleep(50 * time.Millisecond)

	sub.Unsubscribe()

	time.Sleep(200 * time.Millisecond)

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

func TestResourceLeak(t *testing.T) {
	sp := NewSubPub()

	initialGoroutines := runtime.NumGoroutine()

	for i := 0; i < 100; i++ {
		sub, err := sp.Subscribe(fmt.Sprintf("test%d", i%10), func(msg interface{}) {})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}

		err = sp.Publish(fmt.Sprintf("test%d", i%10), "test")
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}

		sub.Unsubscribe()
	}

	time.Sleep(100 * time.Millisecond)

	currentGoroutines := runtime.NumGoroutine()
	if currentGoroutines > initialGoroutines+5 {
		t.Errorf("Potential goroutine leak: started with %d, ended with %d", initialGoroutines, currentGoroutines)
	}

	ctx := context.Background()
	err := sp.Close(ctx)
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestConcurrentSubscribeUnsubscribe(t *testing.T) {
	sp := NewSubPub()

	numGoroutines := 10
	numTopics := 3

	subscribeDone := make(chan struct{}, numGoroutines)
	subscribeErrors := make(chan error, numGoroutines)
	subscribers := make(chan Subscription, numGoroutines)
	unsubscribeDone := make(chan struct{}, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			topic := fmt.Sprintf("topic%d", id%numTopics)

			sub, err := sp.Subscribe(topic, func(msg interface{}) {
			})

			if err != nil {
				subscribeErrors <- err
				return
			}

			subscribers <- sub
			subscribeDone <- struct{}{}
		}(i)
	}

	timeout := time.After(2 * time.Second)
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-subscribeDone:
		case err := <-subscribeErrors:
			t.Errorf("Subscribe error: %v", err)
		case <-timeout:
			t.Fatalf("Timeout waiting for subscriptions")
		}
	}

	close(subscribers)

	var subsList []Subscription
	for sub := range subscribers {
		subsList = append(subsList, sub)
	}

	for i := 0; i < numTopics; i++ {
		topic := fmt.Sprintf("topic%d", i)
		err := sp.Publish(topic, fmt.Sprintf("message for %s", topic))
		if err != nil {
			t.Errorf("Publish error: %v", err)
		}
	}

	time.Sleep(50 * time.Millisecond)

	for i, sub := range subsList {
		go func(id int, s Subscription) {
			s.Unsubscribe()
			unsubscribeDone <- struct{}{}
		}(i, sub)
	}

	timeout = time.After(2 * time.Second)
	for i := 0; i < len(subsList); i++ {
		select {
		case <-unsubscribeDone:
		case <-timeout:
			t.Fatalf("Timeout waiting for unsubscriptions")
		}
	}

	for i := 0; i < numTopics; i++ {
		topic := fmt.Sprintf("topic%d", i)
		err := sp.Publish(topic, fmt.Sprintf("message after unsubscribe for %s", topic))
		if err != nil {
			t.Errorf("Publish after unsubscribe error: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := sp.Close(ctx)
	if err != nil {
		t.Errorf("Close error: %v", err)
	}
}

func TestEmptySubjects(t *testing.T) {
	sp := NewSubPub()

	err := sp.Publish("nonexistent", "test")
	if err != nil {
		t.Errorf("Publish to nonexistent subject failed: %v", err)
	}

	err = sp.Publish("", "test")
	if err != nil {
		t.Errorf("Publish to empty subject failed: %v", err)
	}

	received := make(chan interface{}, 1)
	sub, err := sp.Subscribe("", func(msg interface{}) {
		received <- msg
	})
	if err != nil {
		t.Fatalf("Subscribe to empty subject failed: %v", err)
	}

	err = sp.Publish("", "empty_subject_message")
	if err != nil {
		t.Errorf("Publish to empty subject after subscribe failed: %v", err)
	}

	select {
	case msg := <-received:
		if msg != "empty_subject_message" {
			t.Errorf("Expected 'empty_subject_message', got '%v'", msg)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for message in empty subject")
	}

	sub.Unsubscribe()

	ctx := context.Background()
	err = sp.Close(ctx)
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestMessageOrderMultipleSubscribers(t *testing.T) {
	sp := NewSubPub()

	numMessages := 20

	var results [3][]int
	var resultsMu [3]sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		i := i
		wg.Add(numMessages)

		_, err := sp.Subscribe("test_order", func(msg interface{}) {
			defer wg.Done()

			if num, ok := msg.(int); ok {
				resultsMu[i].Lock()
				results[i] = append(results[i], num)
				resultsMu[i].Unlock()
			}
		})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
	}

	for i := 0; i < numMessages; i++ {
		err := sp.Publish("test_order", i)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message processing")
	}

	for i := 0; i < 3; i++ {
		resultsMu[i].Lock()

		if len(results[i]) != numMessages {
			t.Errorf("Subscriber %d: expected %d messages, got %d", i, numMessages, len(results[i]))
			resultsMu[i].Unlock()
			continue
		}

		for j := 0; j < numMessages; j++ {
			if results[i][j] != j {
				t.Errorf("Subscriber %d: expected message %d at position %d, got %d", i, j, j, results[i][j])
				break
			}
		}

		resultsMu[i].Unlock()
	}

	ctx := context.Background()
	err := sp.Close(ctx)
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}
