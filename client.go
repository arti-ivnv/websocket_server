package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// ClientList is a map to help manage a map of clients
type ClientList map[*Client]bool

// Client is a websocket client, basically a frontend visitor
type Client struct {
	// the websocket connection
	connection *websocket.Conn

	// manager is the used to manage the client
	manager *Manager

	// egress is used to avoid concurrent writes on the WebSocket
	// egress chan []byte
	egress chan Event
}

// NewClient is used to initialize a new Client with all required values initialized
func NewClient(conn *websocket.Conn, manager *Manager) *Client {
	return &Client{
		connection: conn,
		manager:    manager,
		egress:     make(chan Event),
	}
}

// readMessage will start the client to read messages and handle them
// appropriatly
// This is suppose to be run as a goroutine
func (c *Client) readMessages() {
	defer func() {
		// Graceful Close the Connection once this
		// function is done
		c.manager.removeClient(c)
	}()

	// Set Max size of Messages in Bytes
	c.connection.SetReadLimit(1024)

	// Configure Wait time for Pong response, use Current time + pongWait
	// This has to be done here to set the first initial timer.
	if err := c.connection.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Println(err)
		return
	}

	c.connection.SetPongHandler(c.pongHandler)

	// Infinite loop
	for {
		// ReadMessage is used to read the next message is queue
		// in the connection
		_, payload, err := c.connection.ReadMessage()

		if err != nil {
			// If connection is closed, we will recive an error here
			// We only want to log "strange" errors, but not simple disconnection
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error reading a message: %v", err)
			}
			break // Break the loop to close conn and Cleanup
		}
		// log.Println("MessageType; ", messageType)
		// log.Println("Payload: ", string(payload))
		// Marshal incoming data into Event struct
		var request Event
		if err := json.Unmarshal(payload, &request); err != nil {
			log.Printf("error marshaling message: %v", err)
			break // Breaking connection here might be harsh
		}

		if err := c.manager.reouteEvent(request, c); err != nil {
			log.Println("Error handling Message: ", err)
		}

		// Hack to test that WriteMessages works as intended
		// Will be replaced soon
		// for wsclient := range c.manager.clients {
		// 	wsclient.egress <- payload
		// }
	}
}

// pongHandler is useed to handle PongMessages for the Client
func (c *Client) pongHandler(pongMsg string) error {
	// Current time + Pong Wait time
	log.Println("pong")
	return c.connection.SetReadDeadline(time.Now().Add(pongWait))
}

// writeMessages is a process that listens for new messages to output to the Client
func (c *Client) WriteMessages() {

	// Create ticker that triggers a ping at givent interval
	ticker := time.NewTicker(pingInterval)

	defer func() {
		ticker.Stop()

		// Graceful close if this triggers a closing
		c.manager.removeClient(c)
	}()

	for {
		// select is used for waiting on multiple channels (used in goroutines and concurrency).
		// select waits for the first channel to send data.
		// It allows handling multiple channels concurrently.
		select {
		// The arrow operator <- in Go is used for sending and receiving values from channels in concurrent programming.
		// The arrow (<-) points left, meaning we are receiving a value.
		// This blocks execution until a value is available in c.egress.
		case message, ok := <-c.egress:
			// ok will be false incase the egress channel is close
			if !ok {
				// Manager has closed this connection channel, so communicate this to frontend
				if err := c.connection.WriteMessage(websocket.CloseMessage, nil); err != nil {
					// Log that the connection is closed and the reason
					log.Println("connection closed: ", err)
				}
				// Return to close the goroutine
				return
			}

			data, err := json.Marshal(message)
			if err != nil {
				log.Println(err)
				return // closses the connection, should we really
			}

			// Write a regula text to the connection
			if err := c.connection.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Println(err)
			}
			log.Println("sent message")

		case <-ticker.C:
			log.Println("ping")
			// Send the Ping
			if err := c.connection.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Println("writemsg: ", err)
				return // return to break this goroutine triggering cleanup
			}
		}
	}
}
