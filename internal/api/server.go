package api

import (
	"realtimer/internal/config"
	"realtimer/internal/pubsub"

	"github.com/gofiber/fiber/v2"
)

type FiberServer struct {
	*fiber.App
	cfg           config.DBConfig
	pubsubManager *pubsub.SubscriptionManager
}

func New(cfg config.DBConfig, pubsub *pubsub.SubscriptionManager) *FiberServer {

	server := &FiberServer{
		App: fiber.New(fiber.Config{
			ServerHeader: "realtimer",
			AppName:      "realtimer",
		}),
		cfg:           cfg,
		pubsubManager: pubsub,
	}

	return server
}
