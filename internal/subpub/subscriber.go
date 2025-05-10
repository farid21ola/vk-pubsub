package subpub

import (
	"log"
	"sync"
)

type subscriber struct {
	subject string
	handler MessageHandler
	queue   chan interface{}
	done    chan struct{}
	parent  *subPub
	mu      sync.Mutex
	closed  bool
	wg      sync.WaitGroup
	logger  *log.Logger
}

func processRemainingMessages(sub *subscriber) {
	var messages []interface{}

	msgCount := len(sub.queue)
	if msgCount > 0 {
		if sub.logger != nil {
			sub.logger.Printf("Обработка %d оставшихся сообщений для темы %q", msgCount, sub.subject)
		}

		for len(sub.queue) > 0 {
			messages = append(messages, <-sub.queue)
		}

		for i, msg := range messages {
			if sub.logger != nil {
				sub.logger.Printf("Обработка оставшегося сообщения %d из %d для темы %q", i+1, msgCount, sub.subject)
			}
			sub.handler(msg)
		}
	} else if sub.logger != nil {
		sub.logger.Printf("Нет оставшихся сообщений для обработки в теме %q", sub.subject)
	}
}

func (s *subscriber) Unsubscribe() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		if s.logger != nil {
			s.logger.Printf("Подписчик темы %q уже отписан", s.subject)
		}
		return
	}
	s.closed = true
	s.mu.Unlock()

	if s.logger != nil {
		s.logger.Printf("Отписка подписчика темы %q: начало обработки оставшихся сообщений", s.subject)
	}

	processRemainingMessages(s)

	close(s.done)
	close(s.queue)

	if s.logger != nil {
		s.logger.Printf("Отписка подписчика темы %q: ожидание завершения горутины", s.subject)
	}

	s.wg.Wait()

	if s.logger != nil {
		s.logger.Printf("Отписка подписчика темы %q: удаление из списка подписчиков", s.subject)
	}

	s.parent.mu.Lock()
	defer s.parent.mu.Unlock()

	subscribers := s.parent.subscribers[s.subject]
	for i, sub := range subscribers {
		if sub == s {
			s.parent.subscribers[s.subject] = append(subscribers[:i], subscribers[i+1:]...)
			if s.logger != nil {
				s.logger.Printf("Подписчик темы %q удален из системы, осталось подписчиков: %d",
					s.subject, len(s.parent.subscribers[s.subject]))
			}
			break
		}
	}
}
