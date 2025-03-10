// We will begin by building our Manager which is used to serve the connections
// and upgrade regular HTTP requests into a WebSocket connection,
// the manager will also be responsible for keeping track of all Clients.

package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	// pongWait is how long we will await a pong response from client
	pongWait = 10 * time.Second

	// pingInterval has to be less than pongWait. We van can't multiply by 0.9 to get 90% of time
	// Because that can make decimals, so instead *9 / 10 to get 90%
	// The reason why it has to be less than PingFrequency is because otherwise it will send a new Ping before getting a response
	pingInterval = (pongWait * 9) / 10
)

// In this context, var is used to declare a block of variables in Go (Golang).
// The var (...) syntax allows you to define multiple variables at once.
var (
	// websocketUpgrader is used to upgrade incomming HTTP requests into a persistent websocket connection
	websocketUpgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		// remove for production
		// CheckOrigin: func(r *http.Request) bool {
		// 	return true
		// },
		CheckOrigin: checkOrigin,
	}

	ErrEventNotSupported = errors.New("this event type is not supported")
)

// Manager is used to hold references to all Client Registered, Broadcasting etc
type Manager struct {
	clients ClientList

	// Usinga a syncMutex here to be able to lock state before editing clients
	// Could also use Channels to block
	// A read-write mutex that allows multiple readers but only one writer.
	sync.RWMutex

	// handlers are functions that are used to hande Events
	handlers map[string]EventHandler
}

// NewManager is used to initalize all the values inside the manager
func NewManager() *Manager {
	m := &Manager{
		clients:  make(ClientList),
		handlers: make(map[string]EventHandler),
	}
	m.setupEventHandlers()
	return m
}

// checkOrigin will check origin and return true if its allowed
func checkOrigin(r *http.Request) bool {

	// Grab the request origin
	origin := r.Header.Get("Origin")

	switch origin {
	case "http://localhost:8080":
		return true
	default:
		return false
	}
}

// setupEventHandlers configures and adds all handlers
func (m *Manager) setupEventHandlers() {
	m.handlers[EventSendMessage] = func(e Event, c *Client) error {
		fmt.Println(e)
		return nil
	}
}

// routeEvent is used to make sure the correct event goes into the correct handler
func (m *Manager) reouteEvent(event Event, c *Client) error {
	// Check is handler is present in Map
	if handler, ok := m.handlers[event.Type]; ok {
		// Execute the handler and return any err
		if err := handler(event, c); err != nil {
			return err
		}
		return nil
	} else {
		return ErrEventNotSupported
	}
}

// serveWS is a HTTP Handler that has the Manager that allows connections
func (m *Manager) serveWS(w http.ResponseWriter, r *http.Request) {
	log.Println("New connections")
	// Begin by upgrading the HTTP request
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// Create New Client
	client := NewClient(conn, m)

	// Add a newly created client to the manager
	m.addClient(client)

	// start the read / write processes
	// we are going to have two goroutines
	go client.readMessages()
	go client.WriteMessages()

	// We won't do anything yet so close connection again
	// conn.Close()
}

// addClient will add clients to our clientList
func (m *Manager) addClient(client *Client) {
	// Lock so we can manilpulate
	m.Lock()
	// defer will execute a function at the very end
	defer m.Unlock()

	// Add Client
	m.clients[client] = true
}

func (m *Manager) removeClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	// Check is client exists, then delete it
	if _, ok := m.clients[client]; ok {
		// close connection
		client.connection.Close()
		// remove
		delete(m.clients, client)
	}
}
