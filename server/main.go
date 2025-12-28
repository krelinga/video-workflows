package main

import (
	"fmt"
	"log"
	"net/http"

	"go.temporal.io/sdk/client"
)

func main() {
	if err := mainImpl(); err != nil {
		log.Fatal(err)
	}
}

func mainImpl() error {
	// Create Temporal client
	temporalClient, err := client.Dial(client.Options{
		HostPort: client.DefaultHostPort,
	})
	if err != nil {
		return fmt.Errorf("failed to create Temporal client: %w", err)
	}
	defer temporalClient.Close()

	// Create server with library path
	srv := NewServer(temporalClient, "foo/bar")

	// Start HTTP server
	addr := ":8080"
	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
