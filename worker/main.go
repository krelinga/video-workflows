package main

import (
	"fmt"
	"log"

	"github.com/krelinga/video-info/virest"
	"github.com/krelinga/video-transcoder/vtrest"
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

	viClient, err := virest.NewClientWithResponses(fmt.Sprintf("%s:%d", config.VideoInfoHost, config.VideoInfoPort))
	if err != nil {
		return fmt.Errorf("failed to create VideoInfo client: %w", err)
	}
	videoInfoDeps := &vwactivity.VideoInfoDeps{
		Client: viClient,
	}
	w.RegisterActivity(videoInfoDeps.GetVideoInfo)

	tClient, err := vtrest.NewClientWithResponses(fmt.Sprintf("%s:%d", config.TranscodeHost, config.TranscodePort))
	if err != nil {
		return fmt.Errorf("failed to create VTRest client: %w", err)
	}
	transcodeDeps := &vwactivity.TranscodeDeps{
		Client: tClient,
	}
	w.RegisterActivity(transcodeDeps.Transcode)

	// Start worker
	log.Printf("Starting worker on task queue: %s", internal.TaskQueue)
	if err := w.Run(worker.InterruptCh()); err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}

	return nil
}
