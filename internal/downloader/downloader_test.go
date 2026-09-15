package downloader

import (
	"context"
	"errors"
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

func TestDownload_ManyProgressMessages(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "many.sh", `#!/bin/bash
for ((i = 1; i <= 500; i++)); do
  echo "[INFO]: step $i"
done
echo "[ID]: big.mp4"
`)

	var progressMsgs []string
	d := New(script)
	fn, err := d.Download(context.Background(), "https://example.com/video", func(msg string) {
		progressMsgs = append(progressMsgs, msg)
	})
	if err != nil {
		t.Fatal(err)
	}
	if fn != "big.mp4" {
		t.Errorf("got %q, want %q", fn, "big.mp4")
	}
	if len(progressMsgs) != 500 {
		t.Errorf("got %d progress messages, want 500", len(progressMsgs))
	}
}

func TestDownload_ScriptError(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "limit.sh", `#!/bin/bash
echo "[INFO]: Проверяю размер..."
echo "[ERROR]: Видео не влезет в лимит 50MB: оценка ~284.1MB." >&2
echo "[CODE]: size_limit" >&2
exit 1
`)

	d := New(script)
	_, err := d.Download(context.Background(), "https://example.com/video", nil)
	if err == nil {
		t.Fatal("expected error for failing script")
	}

	var scriptErr *ScriptError
	if !errors.As(err, &scriptErr) {
		t.Fatalf("expected ScriptError, got %T: %v", err, err)
	}
	if scriptErr.Code != "size_limit" {
		t.Errorf("got code %q, want %q", scriptErr.Code, "size_limit")
	}
	if scriptErr.Reason != "Видео не влезет в лимит 50MB: оценка ~284.1MB." {
		t.Errorf("got reason %q", scriptErr.Reason)
	}
}

func TestList_Success(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "list.sh", `#!/bin/bash
[[ "${1:-}" == "--list" ]] || { echo "[ERROR]: bad args" >&2; echo "[CODE]: bad_args" >&2; exit 1; }
echo "[MEDIA]: 1|photo"
echo "[MEDIA]: 2|video"
`)

	d := New(script)
	items, err := d.List(context.Background(), "https://www.instagram.com/p/abc/")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Index != 1 || items[0].Kind != "photo" {
		t.Errorf("got %+v", items[0])
	}
	if items[1].Index != 2 || items[1].Kind != "video" {
		t.Errorf("got %+v", items[1])
	}
}

func TestList_ScriptError(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "list_err.sh", `#!/bin/bash
echo "[ERROR]: Не удалось получить список медиа." >&2
echo "[CODE]: media_list_error" >&2
exit 1
`)

	d := New(script)
	_, err := d.List(context.Background(), "https://www.instagram.com/p/abc/")
	if err == nil {
		t.Fatal("expected error for failing script")
	}

	var scriptErr *ScriptError
	if !errors.As(err, &scriptErr) {
		t.Fatalf("expected ScriptError, got %T: %v", err, err)
	}
	if scriptErr.Code != "media_list_error" {
		t.Errorf("got code %q, want %q", scriptErr.Code, "media_list_error")
	}
}

func TestDownloadItem_Photo(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "item.sh", `#!/bin/bash
[[ "${1:-}" == "--item" && "${2:-}" == "2" ]] || { echo "[ERROR]: bad args" >&2; echo "[CODE]: bad_args" >&2; exit 1; }
echo "[INFO]: Скачиваю фото..."
echo "[ID]: abc123.jpg"
`)

	d := New(script)
	var progressMsgs []string
	fn, err := d.DownloadItem(context.Background(), "https://www.instagram.com/p/abc/", 2, func(msg string) {
		progressMsgs = append(progressMsgs, msg)
	})
	if err != nil {
		t.Fatal(err)
	}
	if fn != "abc123.jpg" {
		t.Errorf("got %q, want %q", fn, "abc123.jpg")
	}
	if len(progressMsgs) != 1 || progressMsgs[0] != "Скачиваю фото..." {
		t.Errorf("got progress %v", progressMsgs)
	}
}
