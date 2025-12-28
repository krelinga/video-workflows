package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/krelinga/video-workflows/internal"
	"go.temporal.io/sdk/client"
)

func main() {
	if err := mainImpl(); err != nil {
		log.Fatal(err)
	}
}

func mainImpl() error {
	config := internal.NewServerConfigFromEnv()

	// Create Temporal client
	temporalClient, err := client.Dial(client.Options{
		HostPort: fmt.Sprintf("%s:%d", config.Temporal.Host, config.Temporal.Port),
	})
	if err != nil {
		return fmt.Errorf("failed to create Temporal client: %w", err)
	}
	defer temporalClient.Close()

	// Create server with library path
	srv := NewServer(temporalClient, config.LibraryPath)

	// Start HTTP server
	addr := ":8080"
	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
