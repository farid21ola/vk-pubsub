package subpub

import (
	"context"
	"log"
	"os"
	"sync"
)

type subPub struct {
	subscribers map[string][]*subscriber //[subject][]*subscriber
	mu          sync.RWMutex
	closed      bool
	logger      *log.Logger
}

func NewSubPub() SubPub {
	return &subPub{
		subscribers: make(map[string][]*subscriber),
		logger:      log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile),
	}
}

func NewSubPubWithLogger(logger *log.Logger) SubPub {
	if logger == nil {
		logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
	}
	return &subPub{
		subscribers: make(map[string][]*subscriber),
		logger:      logger,
	}
}

func (s *subPub) Subscribe(subject string, cb MessageHandler) (Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		s.logger.Printf("Ошибка подписки на тему %q: система закрыта", subject)
		return nil, ErrClosed
	}

	sub := &subscriber{
		subject: subject,
		handler: cb,
		queue:   make(chan interface{}, 1000),
		done:    make(chan struct{}),
		parent:  s,
		logger:  s.logger,
	}

	s.subscribers[subject] = append(s.subscribers[subject], sub)
	s.logger.Printf("Добавлен новый подписчик на тему %q, всего подписчиков: %d", subject, len(s.subscribers[subject]))

	sub.wg.Add(1)
	go func() {
		defer sub.wg.Done()
		s.logger.Printf("Запущена горутина для обработки сообщений темы %q", subject)
		for {
			select {
			case <-sub.done:
				s.logger.Printf("Получен сигнал завершения для подписчика темы %q", subject)
				return
			case msg, ok := <-sub.queue:
				if !ok {
					s.logger.Printf("Канал сообщений закрыт для подписчика темы %q", subject)
					return
				}
				s.logger.Printf("Обработка сообщения для темы %q", subject)
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
		s.logger.Printf("Ошибка публикации в тему %q: система закрыта", subject)
		return ErrClosed
	}

	subscribers := s.subscribers[subject]
	if len(subscribers) == 0 {
		s.logger.Printf("Публикация в тему %q: нет подписчиков", subject)
		return nil
	}

	s.logger.Printf("Публикация сообщения в тему %q для %d подписчиков", subject, len(subscribers))
	for i, sub := range subscribers {
		sub.mu.Lock()
		if sub.closed {
			s.logger.Printf("Пропуск закрытого подписчика %d для темы %q", i, subject)
			sub.mu.Unlock()
			continue
		}

		select {
		case sub.queue <- msg:
			s.logger.Printf("Сообщение успешно добавлено в очередь подписчика %d темы %q", i, subject)
		default:
			s.logger.Printf("Очередь подписчика %d темы %q заполнена, сообщение пропущено", i, subject)
			sub.mu.Unlock()
			continue
		}

		sub.mu.Unlock()
	}

	return nil
}

func (s *subPub) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.logger.Printf("Закрытие системы прервано: контекст отменен с ошибкой %v", err)
		return err
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		s.logger.Printf("Система уже закрыта")
		return nil
	}
	s.closed = true
	s.logger.Printf("Система помечена как закрытая, начинаем обработку оставшихся подписчиков")

	var allSubs []*subscriber
	topicCounts := make(map[string]int)
	for topic, subs := range s.subscribers {
		allSubs = append(allSubs, subs...)
		topicCounts[topic] = len(subs)
	}
	totalSubs := len(allSubs)

	s.subscribers = make(map[string][]*subscriber)
	s.mu.Unlock()

	s.logger.Printf("Найдено %d подписчиков для отписки. Распределение по темам: %v", totalSubs, topicCounts)

	for i, sub := range allSubs {
		select {
		case <-ctx.Done():
			s.logger.Printf("Закрытие системы прервано после обработки %d из %d подписчиков: контекст отменен", i, totalSubs)
			return ctx.Err()
		default:
			s.logger.Printf("Отписка подписчика %d из %d (тема %q)", i+1, totalSubs, sub.subject)
			sub.Unsubscribe()
		}
	}

	s.logger.Printf("Система успешно закрыта, все %d подписчиков отписаны", totalSubs)
	return nil
}
