package vwactivity

import (
	"context"
	"fmt"
	"os"
)

type RenameFileParams struct {
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
}

func RenameFile(ctx context.Context, params RenameFileParams) error {
	if params.SourcePath == "" {
		return fmt.Errorf("source_path cannot be empty")
	}
	if params.TargetPath == "" {
		return fmt.Errorf("target_path cannot be empty")
	}

	if err := os.Rename(params.SourcePath, params.TargetPath); err != nil {
		return fmt.Errorf("failed to rename file from %s to %s: %w", params.SourcePath, params.TargetPath, err)
	}

	return nil
}
