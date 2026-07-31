package downloader

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDownload_Success(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "yt_dw.sh", `#!/bin/bash
echo "[ID]: video.mp4"
`)

	d := New(script)
	fn, err := d.Download(context.Background(), "https://example.com/video", nil)
	if err != nil {
		t.Fatal(err)
	}
	if fn != "video.mp4" {
		t.Errorf("got %q, want %q", fn, "video.mp4")
	}
}

func TestDownload_Progress(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "yt_dw.sh", `#!/bin/bash
echo "[INFO]: Downloading..."
echo "[INFO]: Converting..."
echo "[ID]: result.mp4"
`)

	var progressMsgs []string
	d := New(script)
	fn, err := d.Download(context.Background(), "https://example.com/video", func(msg string) {
		progressMsgs = append(progressMsgs, msg)
	})
	if err != nil {
		t.Fatal(err)
	}
	if fn != "result.mp4" {
		t.Errorf("got %q, want %q", fn, "result.mp4")
	}
	if len(progressMsgs) != 2 {
		t.Fatalf("expected 2 progress messages, got %v", progressMsgs)
	}
	if progressMsgs[0] != "Downloading..." {
		t.Errorf("got %q, want %q", progressMsgs[0], "Downloading...")
	}
	if progressMsgs[1] != "Converting..." {
		t.Errorf("got %q, want %q", progressMsgs[1], "Converting...")
	}
}

func TestDownload_ScriptFails(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "fail.sh", `#!/bin/bash
echo "[INFO]: Something went wrong"
exit 1
`)

	d := New(script)
	_, err := d.Download(context.Background(), "https://example.com/video", nil)
	if err == nil {
		t.Fatal("expected error for failing script")
	}
}

func TestDownload_ScriptNotFound(t *testing.T) {
	d := New("/nonexistent/script.sh")
	_, err := d.Download(context.Background(), "https://example.com/video", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent script")
	}
}

func TestDownload_NoID(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "no_id.sh", `#!/bin/bash
echo "[INFO]: No ID here"
echo "some other output"
`)

	d := New(script)
	fn, err := d.Download(context.Background(), "https://example.com/video", nil)
	if err != nil {
		t.Fatal(err)
	}
	if fn != "" {
		t.Errorf("expected empty filename, got %q", fn)
	}
}

func TestDownload_DebugOutput(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "debug.sh", `#!/bin/bash
echo "[INFO]: Downloading..."
echo "some debug line"
echo "[ID]: video.mp4"
echo "another debug line"
`)

	var progressMsgs []string
	d := New(script)
	fn, err := d.Download(context.Background(), "https://example.com/video", func(msg string) {
		progressMsgs = append(progressMsgs, msg)
	})
	if err != nil {
		t.Fatal(err)
	}
	if fn != "video.mp4" {
		t.Errorf("got %q, want %q", fn, "video.mp4")
	}
	if len(progressMsgs) != 1 {
		t.Errorf("expected 1 progress msg, got %v", progressMsgs)
	}
}

func TestDownload_MultipleID(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "multi_id.sh", `#!/bin/bash
echo "[ID]: first.mp4"
echo "[ID]: second.mp4"
`)

	d := New(script)
	fn, err := d.Download(context.Background(), "https://example.com/video", nil)
	if err != nil {
		t.Fatal(err)
	}
	// последний [ID] побеждает
	if fn != "second.mp4" {
		t.Errorf("got %q, want %q", fn, "second.mp4")
	}
}
