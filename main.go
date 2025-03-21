package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
)

func main() {

	// Create a root ctx and a CancelFunc which can be used to cancel retentionMap goroutine
	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)

	defer cancel()

	setupAPI(ctx)

	// Serve on port :8080
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func setupAPI(ctx context.Context) {

	// Create a Manager instance used to handle WebSocket Connections
	manager := NewManager(ctx)

	http.HandleFunc("/login", manager.loginHandler)
	http.HandleFunc("/ws", manager.serveWS)

	http.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, len(manager.clients))
	})

	// Here could be front end handler but I am not gonna use it
}
