package subpub

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrClosed = errors.New("subpub: system is closed")
)

// MessageHandler is a callback function that processes messages delivered to subscribers.
type MessageHandler func(msg interface{})

type Subscription interface {
	// Unsubscribe will remove interest in the current subject subscription is for.
	Unsubscribe()
}

type SubPub interface {
	// Subscribe creates an asynchronous queue subscriber on the given subject.
	Subscribe(subject string, cb MessageHandler) (Subscription, error)

	// Publish publishes the msg argument to the given subject.
	Publish(subject string, msg interface{}) error

	// Close will shutdown sub-pub system.
	// May be blocked by data delivery until the context is canceled.
	Close(ctx context.Context) error
}

type subscriber struct {
	subject string
	handler MessageHandler
	queue   chan interface{}
	done    chan struct{}
	parent  *subPub
	mu      sync.Mutex
	closed  bool
}

type subPub struct {
	subscribers map[string][]*subscriber
	mu          sync.RWMutex
	closed      bool
}

func NewSubPub() SubPub {
	return &subPub{
		subscribers: make(map[string][]*subscriber),
	}
}

func (s *subPub) Subscribe(subject string, cb MessageHandler) (Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, ErrClosed
	}

	// Большой размер буфера для уменьшения вероятности блокировок
	sub := &subscriber{
		subject: subject,
		handler: cb,
		queue:   make(chan interface{}, 1000),
		done:    make(chan struct{}),
		parent:  s,
	}

	s.subscribers[subject] = append(s.subscribers[subject], sub)

	// Запускаем горутину для обработки сообщений
	go func() {
		for {
			select {
			case <-sub.done:
				return
			case msg, ok := <-sub.queue:
				if !ok {
					return
				}
				sub.handler(msg)
			}
		}
	}()

	return sub, nil
}

func (s *subPub) Publish(subject string, msg interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return ErrClosed
	}

	subscribers := s.subscribers[subject]
	for _, sub := range subscribers {
		sub.mu.Lock()
		if sub.closed {
			sub.mu.Unlock()
			continue
		}

		// Заменяем неблокирующую отправку на блокирующую
		select {
		case sub.queue <- msg:
		default:

		}

		sub.mu.Unlock()
	}

	return nil
}

// processRemainingMessages обрабатывает все оставшиеся сообщения в очереди
func processRemainingMessages(sub *subscriber) {
	// Для гарантии сохранения порядка FIFO при отписке,
	// сначала копируем все сообщения из канала в слайс,
	// а затем обрабатываем их в порядке следования
	var messages []interface{}

	// Читаем все сообщения из канала
	for len(sub.queue) > 0 {
		messages = append(messages, <-sub.queue)
	}

	// Обрабатываем сообщения в порядке их получения
	for _, msg := range messages {
		sub.handler(msg)
	}
}

func (s *subscriber) Unsubscribe() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()

	// Обрабатываем все оставшиеся сообщения
	processRemainingMessages(s)

	// Сигнализируем о завершении работы
	close(s.done)

	// Удаляем из списка подписчиков
	s.parent.mu.Lock()
	defer s.parent.mu.Unlock()

	subscribers := s.parent.subscribers[s.subject]
	for i, sub := range subscribers {
		if sub == s {
			s.parent.subscribers[s.subject] = append(subscribers[:i], subscribers[i+1:]...)
			break
		}
	}
}

func (s *subPub) Close(ctx context.Context) error {
	// Проверка контекста на отмену
	if err := ctx.Err(); err != nil {
		// Если контекст уже отменен, просто помечаем систему как закрытую
		// и выходим, оставляя работающие хендлеры активными
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		return err
	}

	// Блокируем для изменения состояния
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true

	// Копируем всех подписчиков для работы с ними без блокировки
	var allSubs []*subscriber
	for _, subs := range s.subscribers {
		allSubs = append(allSubs, subs...)
	}

	// Очищаем карту подписчиков
	s.subscribers = make(map[string][]*subscriber)
	s.mu.Unlock()

	// Обрабатываем всех подписчиков
	for _, sub := range allSubs {
		select {
		case <-ctx.Done():
			// Контекст отменен во время обработки:
			// Выходим сразу, оставляя работающие хендлеры активными
			// Не закрываем каналы, чтобы горутины могли завершить обработку сообщений
			return ctx.Err()
		default:
			// Закрываем подписчика и обрабатываем сообщения
			sub.mu.Lock()
			if !sub.closed {
				sub.closed = true
				sub.mu.Unlock()

				// Обрабатываем оставшиеся сообщения
				for len(sub.queue) > 0 {
					msg := <-sub.queue
					sub.handler(msg)
				}

				// Закрываем каналы
				close(sub.queue)
				close(sub.done)
			} else {
				sub.mu.Unlock()
			}
		}
	}

	return nil
}
