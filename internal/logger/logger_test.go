package logger

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestInit_InvalidLevel_FallsBackToInfo(t *testing.T) {
	var buf bytes.Buffer
	r, w, _ := os.Pipe()
	done := make(chan struct{})
	go func() {
		io.Copy(&buf, r)
		close(done)
	}()

	old := os.Stderr
	os.Stderr = w

	Init("bogus", "text")
	slog.Info("test msg")

	w.Close()
	os.Stderr = old
	<-done

	if !strings.Contains(buf.String(), "test msg") {
		t.Errorf("expected log output to contain \"test msg\", got %q", buf.String())
	}
}

func TestInit_LevelDebug(t *testing.T) {
	Init("debug", "text")
	if !slog.Default().Enabled(nil, slog.LevelDebug) {
		t.Error("expected debug level to be enabled")
	}
}

func TestInit_LevelWarn(t *testing.T) {
	Init("warn", "text")
	if slog.Default().Enabled(nil, slog.LevelDebug) {
		t.Error("expected debug level to be disabled")
	}
	if !slog.Default().Enabled(nil, slog.LevelWarn) {
		t.Error("expected warn level to be enabled")
	}
}

func TestInit_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	r, w, _ := os.Pipe()
	done := make(chan struct{})
	go func() {
		io.Copy(&buf, r)
		close(done)
	}()

	old := os.Stderr
	os.Stderr = w

	Init("info", "json")
	slog.Info("json test")

	w.Close()
	os.Stderr = old
	<-done

	output := buf.String()
	if !strings.HasPrefix(output, "{") {
		t.Errorf("expected JSON output, got %q", output)
	}
	if !strings.Contains(output, `"msg":"json test"`) {
		t.Errorf("expected msg in JSON, got %q", output)
	}
}
