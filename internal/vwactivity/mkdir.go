package vwactivity

import (
	"context"
	"fmt"
	"os"
)

type MkDirParams struct {
	Path string `json:"path"`
}

func MkDir(ctx context.Context, params MkDirParams) error {
	if params.Path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	if err := os.MkdirAll(params.Path, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", params.Path, err)
	}

	return nil
}
