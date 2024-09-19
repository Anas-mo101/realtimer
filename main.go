package main

import (
	"fmt"
	"realtimer/internal/adapters"
	"realtimer/internal/api"
	"realtimer/internal/config"
	"realtimer/internal/pubsub"
)

func main() {
	cfg, err := config.ParseConfig()
	if err != nil {
		panic(err)
	}

	var pubsubManager *pubsub.SubscriptionManager = pubsub.NewSubscriptionManager()

	err = adapters.New(cfg, pubsubManager)
	if err != nil {
		panic(err)
	}

	server := api.New(cfg, pubsubManager)
	server.RegisterFiberRoutes()

	err = server.Listen(fmt.Sprintf(":%d", cfg.Servers.HTTPPort))

	if err != nil {
		panic(fmt.Sprintf("cannot start server: %s", err))
	}
}
