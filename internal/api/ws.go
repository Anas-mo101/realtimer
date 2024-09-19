package api

import (
	"fmt"
	"realtimer/internal/pubsub"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// Middleware to authenticate WebSocket connection using JWT
func authenticateWS(c *fiber.Ctx) error {
	tokenString := c.Query("token") // or get token from headers

	if tokenString == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing token",
		})
	}

	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Make sure the signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).SendString("Invalid token")
	}

	// Extract custom claims from token
	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).SendString("Could not parse claims")
	}

	// Store userID in context locals for later use
	c.Locals("subId", claims.SubID)

	return c.Next()
}

// WebSocket handler to handle event subscriptions
func (s *FiberServer) wsHandler(c *websocket.Conn) {
	defer c.Close()

	event := c.Query("event")
	if event == "" {
		fmt.Println("event param does not exist")
		return
	}

	table := c.Query("table")
	if table == "" {
		fmt.Println("table param does not exist")
		return
	}

	topic := fmt.Sprintf("%s:%s", event, table)
	subId := c.Locals("subId").(string)

	subscriber := pubsub.Subscriber{
		Conn: c,
		Id:   subId,
	}

	s.pubsubManager.Subscribe(topic, subscriber)
	defer s.pubsubManager.Unsubscribe(topic, subscriber)

	fmt.Printf("subsriber %s connected\n", subId)
	for {
		// Read message from client
		_, _, err := c.ReadMessage()
		if err != nil {
			break
		}
	}
}
