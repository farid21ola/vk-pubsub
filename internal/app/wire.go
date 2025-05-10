//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"
)

// InitializeApp выполняет инициализацию всего приложения
func InitializeApp() (*App, error) {
	wire.Build(AppSet)
	return nil, nil
}
