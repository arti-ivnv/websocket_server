package main

import (
	"log"
	"net/http"
)

func main() {

	setupAPI()

	// Serve on port :8080
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func setupAPI() {

	// Create a Manager instance used to handle WebSocket Connections
	manger := NewManager()

	http.HandleFunc("/ws", manger.serveWS)

	// Here could be front end handler but I am not gonna use it
}
