package vwactivity

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/krelinga/video-transcoder/vtrest"
	"go.temporal.io/sdk/activity"
)

type TranscodeParams struct {
	Uuid               string `json:"uuid"`
	InputPath          string `json:"input_path"`
	OutputPath         string `json:"output_path"`
	Profile            string `json:"profile"`
	WebhookCompleteURI string `json:"webhook_complete_uri"`
	WebhookProgressURI string `json:"webhook_progress_uri"`
}

type TranscodeProgress struct {
	Percentage float64 `json:"percentage"`
}

type TranscodeDeps struct {
	Client vtrest.ClientWithResponsesInterface
}

func (d *TranscodeDeps) Transcode(ctx context.Context, params TranscodeParams) error {
	req := vtrest.CreateTranscodeJSONRequestBody{
		Uuid:                uuid.MustParse(params.Uuid),
		SourcePath:          params.InputPath,
		DestinationPath:     params.OutputPath,
		Profile:             params.Profile,
		WebhookToken:        activity.GetInfo(ctx).TaskToken,
		WebhookUri:          &params.WebhookCompleteURI,
		HeartbeatWebhookUri: &params.WebhookProgressURI,
	}
	_, err := d.Client.CreateTranscodeWithResponse(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to initiate transcoding: %w", err)
	}
	return activity.ErrResultPending
}
