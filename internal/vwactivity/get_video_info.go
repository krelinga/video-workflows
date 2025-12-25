package vwactivity

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/krelinga/video-info/virest"
	"go.temporal.io/sdk/activity"
)

type GetVideoInfoParams struct {
	Uuid      string `json:"uuid"`
	VideoPath string `json:"video_path"`
}

type VideoInfo struct {
	DurationSeconds  float64   `json:"duration_seconds"`
	ChapterDurations []float64 `json:"chapter_durations"`
}

type VideoInfoDeps struct {
	Client virest.ClientWithResponsesInterface
}

var ErrGetVideoInfo = errors.New("failed to get video info")

func getVideoInfoError(errorMessage *virest.Error) error {
	return fmt.Errorf("%w: %s", ErrGetVideoInfo, errorMessage.Message)
}

func (d *VideoInfoDeps) GetVideoInfo(ctx context.Context, params GetVideoInfoParams) error {
	req := virest.CreateInfoJSONRequestBody{
		Uuid:      uuid.MustParse(params.Uuid),
		VideoPath: params.VideoPath,
	}
	resp, err := d.Client.CreateInfoWithResponse(ctx, req)
	switch {
	case err != nil:
		return fmt.Errorf("%w: unexpected error: %s", ErrGetVideoInfo, err)
	case resp.JSON201 != nil:
		// Will be completed asynchronously
		return activity.ErrResultPending
	case resp.JSON400 != nil:
		return getVideoInfoError(resp.JSON400)
	case resp.JSON409 != nil:
		return getVideoInfoError(resp.JSON409)
	case resp.JSON500 != nil:
		return getVideoInfoError(resp.JSON500)
	default:
		return fmt.Errorf("%w: unexpected response status %d", ErrGetVideoInfo, resp.HTTPResponse.StatusCode)
	}
}
