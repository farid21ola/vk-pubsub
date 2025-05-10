package config

import (
	"fmt"
	"os"
	"time"

	"github.com/caarlos0/env"
)

type Config struct {
	GRPCHost        string        `env:"GRPC_HOST,required"`
	GRPCPort        int           `env:"GRPC_PORT,required"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT,required"`
}

func NewConfig() *Config {
	cfg := Config{}

	if err := env.Parse(&cfg); err != nil {
		fmt.Printf("config error: %s \n", err)
		os.Exit(1)
	}

	return &cfg
}

func (c *Config) GRPCAddress() string {
	return fmt.Sprintf("%s:%d", c.GRPCHost, c.GRPCPort)
}
