package api

import (
	"fmt"
	"strings"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

func (s *FiberServer) RegisterFiberRoutes() {
	s.App.Post("/api/db", s.callbackHandler)

	s.App.Get("/api/auth", s.authHandler)
	s.App.Use("/api/ws", authenticateWS)
	s.App.Get("/api/ws", websocket.New(s.wsHandler))
}

func (s *FiberServer) callbackHandler(c *fiber.Ctx) error {
	event := c.Queries()["event"]
	if event == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "event param does not exist",
		})
	}

	table := c.Queries()["table"]
	if table == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "table param does not exist",
		})
	}

	body := string(c.Body())
	entries := strings.Split(body, ",")
	keyValueEntries := make(map[string]string)
	for _, entry := range entries {
		keyValue := strings.Split(entry, ": ")
		if len(keyValue) == 2 {
			keyValueEntries[strings.Trim(keyValue[0], " ")] = strings.Trim(keyValue[1], " ")
		}
	}

	/// push keyValueEntries to ws connection
	topic := fmt.Sprintf("%s:%s", strings.ToLower(event), table)
	go s.pubsubManager.Publish(topic, keyValueEntries)

	return nil
}
