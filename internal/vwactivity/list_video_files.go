package vwactivity

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ListVideoFilesParams struct {
	DirectoryPath string `json:"directory_path"`
}

type ListVideoFilesResult struct {
	VideoPaths []string `json:"video_paths"`
}

func ListVideoFiles(ctx context.Context, params ListVideoFilesParams) (*ListVideoFilesResult, error) {
	if params.DirectoryPath == "" {
		return nil, fmt.Errorf("directory_path cannot be empty")
	}

	// Check if directory exists
	info, err := os.Stat(params.DirectoryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to access directory %s: %w", params.DirectoryPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", params.DirectoryPath)
	}

	// Read directory entries
	entries, err := os.ReadDir(params.DirectoryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", params.DirectoryPath, err)
	}

	// Filter for .mkv files
	var videoPaths []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".mkv") {
			videoPath := filepath.Join(params.DirectoryPath, entry.Name())
			videoPaths = append(videoPaths, videoPath)
		}
	}

	return &ListVideoFilesResult{
		VideoPaths: videoPaths,
	}, nil
}
