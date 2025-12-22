package vwdisc

import (
	"fmt"
	"path/filepath"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/krelinga/video-workflows/internal/vwactivity"
)

type Params struct {
	UUID        string `json:"uuid"`
	Path        string `json:"path"`
	LibraryPath string `json:"library_path"`
}

type State struct {
	DirectoryMoved bool                 `json:"directory_moved"`
	Files          map[string]FileState `json:"files,omitempty"`
}

type FileState struct {
	ScreenshotPath *string       `json:"screenshot_path,omitempty"`
	Category       *FileCategory `json:"category,omitempty"`
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

	// TODO: List all the files in the renamed directory and create corresponding state entries.

	// TODO: For each file, generate a screenshot and update the state.

	// TODO: Wait for the user to categorize each file and update the state.

	// TODO: Move each file to its final location based on its category.

	// TODO: Start transcoding jobs for main title.

	return state, nil
}
