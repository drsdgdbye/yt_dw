package filestore

import (
	"io"
	"os"
	"testing"
)

func TestOpen_NotFound(t *testing.T) {
	s := New("/nonexistent/")
	_, err := s.Open("test.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestOpen_Success(t *testing.T) {
	dir := t.TempDir()
	s := New(dir + "/")

	path := dir + "/test.txt"
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	r, err := s.Open("test.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", string(data), "hello")
	}
}

func TestRemove_NotFound(t *testing.T) {
	s := New("/nonexistent/")
	err := s.Remove("test.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestRemove_Success(t *testing.T) {
	dir := t.TempDir()
	s := New(dir + "/")

	path := dir + "/test.txt"
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := s.Remove("test.txt"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should have been removed")
	}
}
