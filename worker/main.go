package main

import (
	"fmt"
	"log"

	"github.com/krelinga/video-workflows/internal"
	"github.com/krelinga/video-workflows/internal/vwactivity"
	"github.com/krelinga/video-workflows/internal/workflows/vwdisc"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	if err := mainImpl(); err != nil {
		log.Fatal(err)
	}
}

func mainImpl() error {
	// Load configuration from environment
	config := internal.NewWorkerConfigFromEnv()

	// Create Temporal client
	temporalClient, err := client.Dial(client.Options{
		HostPort: fmt.Sprintf("%s:%d", config.Temporal.Host, config.Temporal.Port),
	})
	if err != nil {
		return fmt.Errorf("failed to create Temporal client: %w", err)
	}
	defer temporalClient.Close()

	// Create worker
	w := worker.New(temporalClient, internal.TaskQueue, worker.Options{})

	// Register workflows
	w.RegisterWorkflow(vwdisc.Workflow)

	// Register activities
	w.RegisterActivity(vwactivity.RenameFile)

	// Register activities with dependencies
	// Note: GetVideoInfo and Transcode require external clients which should be
	// injected via their respective Deps structs. For now, registering with nil
	// dependencies - these should be properly initialized in production.
	videoInfoDeps := &vwactivity.VideoInfoDeps{
		Client: nil, // TODO: Initialize virest.Client
	}
	w.RegisterActivity(videoInfoDeps.GetVideoInfo)

	transcodeDeps := &vwactivity.TranscodeDeps{
		Client: nil, // TODO: Initialize vtrest.Client
	}
	w.RegisterActivity(transcodeDeps.Transcode)

	// Start worker
	log.Printf("Starting worker on task queue: %s", internal.TaskQueue)
	if err := w.Run(worker.InterruptCh()); err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}

	return nil
}
