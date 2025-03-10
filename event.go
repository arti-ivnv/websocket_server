package main

import "encoding/json"

// Event is the messages sent over the websocket
// Used to differ between different actions
type Event struct {
	// Type is the message type sent
	Type string `json:"type"`
	// Payload is the data based on the type
	Payload json.RawMessage `json:"payload"`
}

// EventHandler is a function signature that is used to affect messages on the socket and triggered
type EventHandler func(event Event, c *Client) error

const (
	// EventSendMessage is the event name for new chaat messages sent
	EventSendMessage = "send_message"
)

// SendMessageEvent is the payload sent in the
// send_message event
type SendMessageEvent struct {
	Message string `json:"message"`
	From    string `json:"from"`
}
