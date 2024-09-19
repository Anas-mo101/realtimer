package pubsub

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/gofiber/contrib/websocket"
)

type Subscriber struct {
	Conn *websocket.Conn
	Id   string
}

type SubscriptionManager struct {
	subscribers map[string][]Subscriber // map of topic to slice of WebSocket connections
	mu          sync.RWMutex            // to handle concurrent access
}

func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		subscribers: make(map[string][]Subscriber),
	}
}

func (s *SubscriptionManager) Subscribe(topic string, sub Subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add the client to the list of subscribers for the topic
	s.subscribers[topic] = append(s.subscribers[topic], sub)
}

func (s *SubscriptionManager) Unsubscribe(topic string, sub Subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove the client from the list of subscribers for the topic
	for i, c := range s.subscribers[topic] {
		if c == sub {
			s.subscribers[topic] = append(s.subscribers[topic][:i], s.subscribers[topic][i+1:]...)
			break
		}
	}
}

func (s *SubscriptionManager) Publish(topic string, message map[string]string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Convert map to JSON
	jsonData, err := json.Marshal(message)
	if err != nil {
		fmt.Println("Error converting map to JSON:", err)
		return
	}

	// Send the message to all clients subscribed to this topic
	for _, sub := range s.subscribers[topic] {
		if err := sub.Conn.WriteMessage(websocket.TextMessage, jsonData); err != nil {
			log.Printf("error writing message to topic %s: %v", topic, err)
		}
	}
}
