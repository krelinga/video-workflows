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
	PreviewPath    string `json:"preview_path"`
	WebhookBaseURI string `json:"webhook_base_uri"`
}

type State struct {
	DirectoryMoved     bool                 `json:"directory_moved"`
	Files              map[string]FileState `json:"files,omitempty"`
	FilesListed        bool                 `json:"files_listed"`
	GotFileDiagnostics bool                 `json:"got_file_diagnostics"`
}

type FileState struct {
	DurationSeconds         *float64      `json:"duration_seconds,omitempty"`
	ChapterDurationsSeconds []float64     `json:"chapter_durations_seconds,omitempty"`
	PreviewPath             *string       `json:"preview_path,omitempty"`
	Category                *FileCategory `json:"category,omitempty"`
	PreviewError            *string       `json:"preview_error,omitempty"`
	InfoError               *string       `json:"info_error,omitempty"`
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

	// Create preview directory
	makePreviewDirCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	})
	previewDir := filepath.Join(params.PreviewPath, params.UUID)
	makePreviewDirParams := vwactivity.MkDirParams{
		Path: previewDir,
	}
	if err := workflow.ExecuteActivity(makePreviewDirCtx, vwactivity.MkDir, makePreviewDirParams).Get(makePreviewDirCtx, nil); err != nil {
		return state, fmt.Errorf("failed to create preview directory: %w", err)
	}

	// For each file, get it's info & start generating a preview.  Update status.
	getVideoInfoCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
	})
	generatePreviewCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
	})
	diagSelect := workflow.NewSelector(ctx)
	diagCount := 0
	for videoPath := range state.Files {
		var videoInfoUuid string
		if err := workflow.SideEffect(ctx, newUUID).Get(&videoInfoUuid); err != nil {
			return state, fmt.Errorf("failed to generate UUID for video info activity: %w", err)
		}
		videoInfoParams := vwactivity.GetVideoInfoParams{
			Uuid:               videoInfoUuid,
			VideoPath:          videoPath,
			WebhookCompleteURI: params.WebhookBaseURI + "/get_video_info/complete",
		}
		var infoDeps *vwactivity.VideoInfoDeps
		videoInfoFuture := workflow.ExecuteActivity(getVideoInfoCtx, infoDeps.GetVideoInfo, videoInfoParams)
		diagCount++
		diagSelect.AddFuture(videoInfoFuture, func(f workflow.Future) {
			var info vwactivity.VideoInfo
			err := f.Get(getVideoInfoCtx, &info)
			if err != nil {
				logger.Error("Failed to get video info", "videoPath", videoPath, "error", err)
				return
			}
			fileState := state.Files[videoPath]
			fileState.DurationSeconds = &info.DurationSeconds
			fileState.ChapterDurationsSeconds = info.ChapterDurations
			state.Files[videoPath] = fileState
		})

		var previewUuid string
		if err := workflow.SideEffect(ctx, newUUID).Get(&previewUuid); err != nil {
			return state, fmt.Errorf("failed to generate UUID for preview activity: %w", err)
		}
		videoExt := filepath.Ext(videoPath)
		var previewBase string
		if videoExt == "" {
			previewBase = filepath.Base(videoPath) + ".mp4"
		} else {
			previewBase = filepath.Base(videoPath[0:len(videoPath)-len(videoExt)]) + ".mp4"
		}
		previewParams := vwactivity.TranscodeParams{
			Uuid:               previewUuid,
			InputPath:          videoPath,
			OutputPath:         filepath.Join(previewDir, previewBase),
			Profile:            "preview",
			WebhookCompleteURI: params.WebhookBaseURI + "/transcode/complete",
			WebhookProgressURI: params.WebhookBaseURI + "/transcode/progress",
		}
		var transcodeDeps *vwactivity.TranscodeDeps
		previewFuture := workflow.ExecuteActivity(generatePreviewCtx, transcodeDeps.Transcode, previewParams)
		diagCount++
		diagSelect.AddFuture(previewFuture, func(f workflow.Future) {
			err := f.Get(generatePreviewCtx, nil)
			if err != nil {
				logger.Error("Failed to generate preview", "videoPath", videoPath, "error", err)
				return
			}
			fileState := state.Files[videoPath]
			fileState.PreviewPath = &previewParams.OutputPath
			state.Files[videoPath] = fileState
		})
	}
	for range diagCount {
		diagSelect.Select(ctx)
	}
	state.GotFileDiagnostics = true

	// TODO: Wait for the user to categorize each file and update the state.

	// TODO: Move each file to its final location based on its category.

	// TODO: Transcode main title.

	return state, nil
}

func newUUID(ctx workflow.Context) any {
	return uuid.New().String()
}
