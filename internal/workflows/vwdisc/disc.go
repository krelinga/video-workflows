package vwdisc

import (
	"fmt"
	"path/filepath"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/google/uuid"
	"github.com/krelinga/video-workflows/internal/vwactivity"
)

type Params struct {
	UUID           string `json:"uuid"`
	Path           string `json:"path"`
	LibraryPath    string `json:"library_path"`
	WebhookBaseURI string `json:"webhook_base_uri"`
}

type State struct {
	DirectoryMoved   bool                 `json:"directory_moved"`
	Files            map[string]FileState `json:"files,omitempty"`
	FilesListed      bool                 `json:"files_listed"`
	GotFileDurations bool                 `json:"got_file_durations"`
}

type FileState struct {
	DurationSeconds         float64       `json:"duration_seconds,omitempty"`
	ChapterDurationsSeconds []float64     `json:"chapter_durations_seconds,omitempty"`
	ScreenshotPath          *string       `json:"screenshot_path,omitempty"`
	Category                *FileCategory `json:"category,omitempty"`
}

type FileCategory string

const (
	FileCategoryMainTitle FileCategory = "main_title"
	FileCategoryExtra     FileCategory = "extra"
	FileCategoryJunk      FileCategory = "junk"
)

const QueryGetState = "GetState"

func Workflow(ctx workflow.Context, params Params) (State, error) {
	// Set up state and an associated query handler.
	var state State
	stateQuery := func() (State, error) {
		return state, nil
	}
	if err := workflow.SetQueryHandler(ctx, QueryGetState, stateQuery); err != nil {
		return state, fmt.Errorf("failed to set query handler: %w", err)
	}

	// Move the directory.
	libraryPath := filepath.Join(params.LibraryPath, params.UUID)
	renameFileOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	renameFileParams := vwactivity.RenameFileParams{
		SourcePath: params.Path,
		TargetPath: libraryPath,
	}
	renameFileCtx := workflow.WithActivityOptions(ctx, renameFileOptions)
	if err := workflow.ExecuteActivity(renameFileCtx, vwactivity.RenameFile, renameFileParams).Get(renameFileCtx, nil); err != nil {
		return state, fmt.Errorf("failed to move directory: %w", err)
	}
	state.DirectoryMoved = true

	// List all the files in the renamed directory and create corresponding state entries.
	listVideoFilesCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	})
	listVideoFilesParams := vwactivity.ListVideoFilesParams{
		DirectoryPath: libraryPath,
	}
	logger := workflow.GetLogger(ctx)
	logger.Info("Listing video files", "directory", listVideoFilesParams.DirectoryPath)
	var listVideoFilesResult vwactivity.ListVideoFilesResult
	if err := workflow.ExecuteActivity(listVideoFilesCtx, vwactivity.ListVideoFiles, listVideoFilesParams).Get(listVideoFilesCtx, &listVideoFilesResult); err != nil {
		return state, fmt.Errorf("failed to list video files: %w", err)
	}
	logger.Info("Listed video files", "files", listVideoFilesResult.VideoPaths)
	state.Files = make(map[string]FileState)
	for _, videoPath := range listVideoFilesResult.VideoPaths {
		state.Files[videoPath] = FileState{}
	}
	state.FilesListed = true

	// For each file, get it's info and update state.
	getVideoInfoCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
	})
	videoInfoSelect := workflow.NewSelector(ctx)
	for videoPath := range state.Files {
		var uuid string
		if err := workflow.SideEffect(ctx, newUUID).Get(&uuid); err != nil {
			return state, fmt.Errorf("failed to generate UUID for video info activity: %w", err)
		}
		params := vwactivity.GetVideoInfoParams{
			Uuid:               uuid,
			VideoPath:          videoPath,
			WebhookCompleteURI: params.WebhookBaseURI + "/get_video_info/complete",
		}
		var infoDeps *vwactivity.VideoInfoDeps
		future := workflow.ExecuteActivity(getVideoInfoCtx, infoDeps.GetVideoInfo, params)
		videoInfoSelect.AddFuture(future, func(f workflow.Future) {
			var info vwactivity.VideoInfo
			err := f.Get(getVideoInfoCtx, &info)
			if err != nil {
				logger.Error("Failed to get video info", "videoPath", videoPath, "error", err)
				return
			}
			fileState := state.Files[videoPath]
			fileState.DurationSeconds = info.DurationSeconds
			fileState.ChapterDurationsSeconds = info.ChapterDurations
			state.Files[videoPath] = fileState
		})
	}
	for range state.Files {
		videoInfoSelect.Select(ctx)
	}
	state.GotFileDurations = true

	// TODO: For each file, generate a screenshot and update the state.

	// TODO: Wait for the user to categorize each file and update the state.

	// TODO: Move each file to its final location based on its category.

	// TODO: Transcode main title.

	return state, nil
}

func newUUID(ctx workflow.Context) any {
	return uuid.New().String()
}
