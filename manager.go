// We will begin by building our Manager which is used to serve the connections
// and upgrade regular HTTP requests into a WebSocket connection,
// the manager will also be responsible for keeping track of all Clients.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
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

	// otps is a map of allowed OTP to accept connections from
	otps RetentionMap
}

// NewManager is used to initalize all the values inside the manager
func NewManager(ctx context.Context) *Manager {
	m := &Manager{
		clients:  make(ClientList),
		handlers: make(map[string]EventHandler),

		// Create a new retentionMap that remove OTPS older than 5 senconds
		otps: NewRetentionMap(ctx, 20*time.Second),
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
		return true
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

	// Grab the OTP int the Get param
	otp := r.URL.Query().Get("otp")
	fmt.Println(otp)
	if otp == "" {
		// Tell the user its not authorized
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Verify OTP is existing
	if !m.otps.VerifyOTP(otp) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

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
	go client.writeMessages()

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

// loginHandler is used to verify user authentication and return one time password
func (m *Manager) loginHandler(w http.ResponseWriter, r *http.Request) {

	type userLoginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	var req userLoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		// fmt.Println(err.Error())
		return
	}

	// Authenticate user / Verify Access token, what ever auth method you use
	if req.Username == "arti" && req.Password == "123" {
		// format to return otp into the frontend
		type response struct {
			OTP string `json:"otp"`
		}

		// add a new OTP
		otp := m.otps.NewOTP()

		resp := response{
			OTP: otp.Key,
		}

		data, err := json.Marshal(resp)
		if err != nil {
			log.Println(err)
			return
		}

		// Return a response to the Authenticated user with the OTP
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return

	}

	// Failer to auth
	w.WriteHeader(http.StatusUnauthorized)
}
