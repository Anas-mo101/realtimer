package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

type AuthResponse struct {
	Token string `json:"token"`
}

func main() {
	// Step 1: Authenticate and get token
	token, err := getAuthToken("1234") // Replace "1234" with your actual id
	if err != nil {
		log.Fatalf("Failed to get auth token: %v", err)
	}
	fmt.Printf("Got token: %s\n", token)

	// Step 2: Use the token to connect to WebSocket and subscribe to events
	err = subscribeToEvents(token, "insert", "categories")
	if err != nil {
		log.Fatalf("Failed to subscribe to events: %v", err)
	}
}

func getAuthToken(id string) (string, error) {
	// Define the request URL with the id as a query parameter
	authURL := fmt.Sprintf("http://127.0.0.1:8080/api/auth?id=%s", id) // Adjust the URL as needed

	// Make the GET request to /auth/api
	resp, err := http.Get(authURL)
	if err != nil {
		return "", fmt.Errorf("error making GET request: %w", err)
	}
	defer resp.Body.Close()

	// Read and parse the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	// Parse the JSON response to get the token
	var authResp AuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return "", fmt.Errorf("error unmarshalling auth response: %w", err)
	}

	return authResp.Token, nil
}

func subscribeToEvents(token, event, table string) error {
	// Build the WebSocket URL with token, event, and table query parameters
	wsURL := url.URL{
		Scheme:   "ws",
		Host:     "127.0.0.1:8080", // Replace with actual server host
		Path:     "/api/ws",
		RawQuery: fmt.Sprintf("token=%s&event=%s&table=%s", token, event, table),
	}

	// Establish a WebSocket connection
	conn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		return fmt.Errorf("error connecting to WebSocket: %w", err)
	}
	defer conn.Close()

	fmt.Printf("Connected to WebSocket at %s\n", wsURL.String())

	// Listen for messages from the WebSocket
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("error reading WebSocket message: %w", err)
		}
		fmt.Printf("Received message: %s\n", message)
	}
}
