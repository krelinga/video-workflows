package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/krelinga/video-workflows/internal"
	"github.com/krelinga/video-workflows/internal/vwactivity"
	"github.com/krelinga/video-workflows/internal/workflows/vwdisc"
	"github.com/krelinga/video-workflows/vwrest"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

// Server implements the vwrest.StrictServerInterface for handling disc workflow REST API requests.
type Server struct {
	temporalClient client.Client
	libraryPath    string
}

// NewServer creates a new Server with the given Temporal client and library path.
func NewServer(temporalClient client.Client, libraryPath string) *Server {
	return &Server{
		temporalClient: temporalClient,
		libraryPath:    libraryPath,
	}
}

// Handler returns an http.Handler that routes requests to the server implementation.
func (s *Server) Handler() http.Handler {
	return vwrest.Handler(vwrest.NewStrictHandler(s, nil))
}

// CreateDisc starts a new disc workflow with the given UUID and path.
func (s *Server) CreateDisc(ctx context.Context, request vwrest.CreateDiscRequestObject) (vwrest.CreateDiscResponseObject, error) {
	params := vwdisc.Params{
		UUID:        request.Body.Uuid.String(),
		Path:        request.Body.Path,
		LibraryPath: s.libraryPath,
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        request.Body.Uuid.String(),
		TaskQueue: internal.TaskQueue,
	}

	_, err := s.temporalClient.ExecuteWorkflow(ctx, workflowOptions, vwdisc.Workflow, params)
	if err != nil {
		var alreadyStartedErr *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStartedErr) {
			return vwrest.CreateDisc409JSONResponse{
				Code:    "CONFLICT",
				Message: fmt.Sprintf("workflow with UUID %s already exists", request.Body.Uuid.String()),
			}, nil
		}
		return vwrest.CreateDisc500JSONResponse{
			Code:    "INTERNAL_ERROR",
			Message: fmt.Sprintf("failed to start workflow: %v", err),
		}, nil
	}

	return vwrest.CreateDisc201JSONResponse{
		Uuid:   request.Body.Uuid,
		Path:   request.Body.Path,
		Status: "created",
	}, nil
}

// CompleteGetVideoInfoActivity completes a GetVideoInfo activity with the provided result or error.
func (s *Server) CompleteGetVideoInfoActivity(ctx context.Context, request vwrest.CompleteGetVideoInfoActivityRequestObject) (vwrest.CompleteGetVideoInfoActivityResponseObject, error) {
	if request.Body.Error != nil {
		err := s.temporalClient.CompleteActivity(ctx, request.Body.Token, nil, errors.New(*request.Body.Error))
		if err != nil {
			return vwrest.CompleteGetVideoInfoActivity500JSONResponse{
				Code:    "INTERNAL_ERROR",
				Message: fmt.Sprintf("failed to complete activity with error: %v", err),
			}, nil
		}
		return vwrest.CompleteGetVideoInfoActivity200Response{}, nil
	}

	if request.Body.Result == nil {
		return vwrest.CompleteGetVideoInfoActivity400JSONResponse{
			Code:    "BAD_REQUEST",
			Message: "either result or error must be provided",
		}, nil
	}

	var durationSeconds float64
	if request.Body.Result.TotalDurationSeconds != nil {
		durationSeconds = *request.Body.Result.TotalDurationSeconds
	}

	result := vwactivity.VideoInfo{
		DurationSeconds:  durationSeconds,
		ChapterDurations: request.Body.Result.ChapterDurationsSeconds,
	}

	err := s.temporalClient.CompleteActivity(ctx, request.Body.Token, result, nil)
	if err != nil {
		return vwrest.CompleteGetVideoInfoActivity500JSONResponse{
			Code:    "INTERNAL_ERROR",
			Message: fmt.Sprintf("failed to complete activity: %v", err),
		}, nil
	}

	return vwrest.CompleteGetVideoInfoActivity200Response{}, nil
}

// CompleteTranscodeActivity completes a Transcode activity with success or an error.
func (s *Server) CompleteTranscodeActivity(ctx context.Context, request vwrest.CompleteTranscodeActivityRequestObject) (vwrest.CompleteTranscodeActivityResponseObject, error) {
	if request.Body.Error != nil {
		err := s.temporalClient.CompleteActivity(ctx, request.Body.Token, nil, errors.New(*request.Body.Error))
		if err != nil {
			return vwrest.CompleteTranscodeActivity500JSONResponse{
				Code:    "INTERNAL_ERROR",
				Message: fmt.Sprintf("failed to complete activity with error: %v", err),
			}, nil
		}
		return vwrest.CompleteTranscodeActivity200Response{}, nil
	}

	err := s.temporalClient.CompleteActivity(ctx, request.Body.Token, nil, nil)
	if err != nil {
		return vwrest.CompleteTranscodeActivity500JSONResponse{
			Code:    "INTERNAL_ERROR",
			Message: fmt.Sprintf("failed to complete activity: %v", err),
		}, nil
	}

	return vwrest.CompleteTranscodeActivity200Response{}, nil
}

// TranscodeActivityHeartbeat records a heartbeat for an in-progress Transcode activity.
func (s *Server) TranscodeActivityHeartbeat(ctx context.Context, request vwrest.TranscodeActivityHeartbeatRequestObject) (vwrest.TranscodeActivityHeartbeatResponseObject, error) {
	var progress vwactivity.TranscodeProgress
	if request.Body.Progress != nil {
		progress.Percentage = float64(*request.Body.Progress)
	}

	err := s.temporalClient.RecordActivityHeartbeat(ctx, request.Body.Token, progress)
	if err != nil {
		return vwrest.TranscodeActivityHeartbeat500JSONResponse{
			Code:    "INTERNAL_ERROR",
			Message: fmt.Sprintf("failed to record heartbeat: %v", err),
		}, nil
	}

	return vwrest.TranscodeActivityHeartbeat200Response{}, nil
}
