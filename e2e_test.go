package videoworkflows

import (
	"os"
	"testing"

	"golang.org/x/mod/modfile"
)

func readModfile(t *testing.T) *modfile.File {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}

	modFile, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		t.Fatalf("failed to parse go.mod: %v", err)
	}

	return modFile
}

func transcodeTag(t *testing.T) string {
	modFile := readModfile(t)

	for _, req := range modFile.Require {
		if req.Mod.Path == "github.com/krelinga/video-transcoder" {
			return req.Mod.Version
		}
	}

	t.Fatal("github.com/krelinga/video-transcoder not found in go.mod")
	return ""
}

func videoInfoTag(t *testing.T) string {
	modFile := readModfile(t)

	for _, req := range modFile.Require {
		if req.Mod.Path == "github.com/krelinga/video-info" {
			return req.Mod.Version
		}
	}

	t.Fatal("github.com/krelinga/video-info not found in go.mod")
	return ""
}

func TestEnd2End(t *testing.T) {
	t.Logf("Using video-transcoder version: %s", transcodeTag(t))
	t.Logf("Using video-info version: %s", videoInfoTag(t))
}