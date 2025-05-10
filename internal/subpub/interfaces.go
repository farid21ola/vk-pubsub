package subpub

import (
	"context"
	"errors"
)

var (
	// ErrClosed возвращается, когда операция вызывается на закрытой системе
	ErrClosed = errors.New("subpub: system is closed")
)

// MessageHandler - функция обработки сообщений, доставляемых подписчикам
type MessageHandler func(msg interface{})

// Subscription представляет подписку на определенную тему
type Subscription interface {
	// Unsubscribe отменяет подписку на текущую тему
	Unsubscribe()
}

// SubPub представляет систему публикации-подписки
type SubPub interface {
	// Subscribe создает асинхронного подписчика с очередью для указанной темы
	Subscribe(subject string, cb MessageHandler) (Subscription, error)

	// Publish публикует сообщение msg для указанной темы
	Publish(subject string, msg interface{}) error

	// Close выключает систему публикации-подписки
	// Может блокироваться до доставки данных пока контекст не будет отменен
	Close(ctx context.Context) error
}
