package main

import (
	"fmt"
	"realtimer/internal/adapters"
	"realtimer/internal/api"
	"realtimer/internal/config"
)

func main() {
	cfg, err := config.ParseConfig()
	if err != nil {
		panic(err)
	}

	err = adapters.New(cfg)
	if err != nil {
		panic(err)
	}

	server := api.New(cfg)
	server.RegisterFiberRoutes()

	err = server.Listen(fmt.Sprintf(":%d", cfg.Servers.HTTPPort))

	if err != nil {
		panic(fmt.Sprintf("cannot start server: %s", err))
	}
}
