package telegram

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yt_dw/internal/downloader"
	"yt_dw/internal/filestore"
	"yt_dw/internal/stats"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m *mockFileInfo) ModTime() time.Time { return m.modTime }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

type statReadCloser struct {
	io.ReadCloser
	size int64
}

func (s *statReadCloser) Stat() (os.FileInfo, error) {
	return &mockFileInfo{size: s.size}, nil
}

func newTestHandler(t *testing.T, d downloader.Downloader, s filestore.FileStore) *Handler {
	t.Helper()
	st := stats.New(filepath.Join(t.TempDir(), "stats.json"))
	return NewHandler(d, s, st, nil)
}

func TestHandler_Start_NilMessage(t *testing.T) {
	h := newTestHandler(t, &mockDownloader{}, &mockFileStore{})
	h.Start(context.Background(), nil, &models.Update{})
}

func TestHandler_Default_NilMessage(t *testing.T) {
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			t.Error("unexpected call for nil message")
			return nil, nil
		},
	}
	h := newTestHandler(t, &mockDownloader{}, &mockFileStore{})
	h.Default(context.Background(), b, &models.Update{})
}

func TestHandler_Default_SendsMessage(t *testing.T) {
	var called bool
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			called = true
			if params.ChatID != int64(42) {
				t.Errorf("got chatID %v, want 42", params.ChatID)
			}
			return &models.Message{}, nil
		},
	}
	h := newTestHandler(t, &mockDownloader{}, &mockFileStore{})
	h.Default(context.Background(), b, &models.Update{
		Message: &models.Message{Chat: models.Chat{ID: 42}},
	})
	if !called {
		t.Fatal("SendMessage was not called")
	}
}

func TestHandler_Start_Success(t *testing.T) {
	var called bool
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			called = true
			if params.ChatID != int64(42) {
				t.Errorf("got chatID %v, want 42", params.ChatID)
			}
			if params.Text != "Стартуем 🚀" {
				t.Errorf("got text %q", params.Text)
			}
			return &models.Message{}, nil
		},
	}
	h := newTestHandler(t, &mockDownloader{}, &mockFileStore{})
	h.Start(context.Background(), b, &models.Update{
		Message: &models.Message{Chat: models.Chat{ID: 42}},
	})
	if !called {
		t.Fatal("SendMessage was not called")
	}
}

func TestHandler_Link_NilMessage(t *testing.T) {
	h := newTestHandler(t, &mockDownloader{}, &mockFileStore{})
	h.Link(context.Background(), nil, &models.Update{})
}

func TestHandler_Link_SendMessageFails(t *testing.T) {
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			return nil, errors.New("send error")
		},
	}
	h := newTestHandler(t, &mockDownloader{}, &mockFileStore{})
	h.Link(context.Background(), b, &models.Update{
		Message: &models.Message{Text: "https://youtube.com/watch?v=test"},
	})
}

func TestHandler_Link_NilMessageResponse(t *testing.T) {
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			return nil, nil
		},
	}
	h := newTestHandler(t, &mockDownloader{}, &mockFileStore{})
	h.Link(context.Background(), b, &models.Update{
		Message: &models.Message{Text: "https://youtube.com/watch?v=test"},
	})
}

func TestHandler_Link_InvalidURL(t *testing.T) {
	var errorEdit string
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			return &models.Message{ID: 1, Chat: models.Chat{ID: 100}}, nil
		},
		editMessageTextFn: func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
			errorEdit = params.Text
			return &models.Message{}, nil
		},
	}
	h := newTestHandler(t, &mockDownloader{}, &mockFileStore{})
	h.Link(context.Background(), b, &models.Update{
		Message: &models.Message{Text: "not a url"},
	})
	if errorEdit != "Ссылка не валидна 😔" {
		t.Errorf("got edit %q", errorEdit)
	}
}

func TestHandler_Link_DownloadFails(t *testing.T) {
	var edits []string
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			return &models.Message{ID: 1, Chat: models.Chat{ID: 100}}, nil
		},
		editMessageTextFn: func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
			edits = append(edits, params.Text)
			return &models.Message{}, nil
		},
	}
	dl := &mockDownloader{
		downloadFn: func(ctx context.Context, link string, progress func(string)) (string, error) {
			return "", errors.New("download failed")
		},
	}
	h := newTestHandler(t, dl, &mockFileStore{})
	h.Link(context.Background(), b, &models.Update{
		Message: &models.Message{Text: "https://youtube.com/watch?v=test"},
	})
	if len(edits) != 1 || edits[0] != "Что-то пошло не так" {
		t.Errorf("expected 'Что-то пошло не так', got %v", edits)
	}
}

func TestHandler_Link_PartFile(t *testing.T) {
	var edits []string
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			return &models.Message{ID: 1, Chat: models.Chat{ID: 100}}, nil
		},
		editMessageTextFn: func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
			edits = append(edits, params.Text)
			return &models.Message{}, nil
		},
	}
	dl := &mockDownloader{
		downloadFn: func(ctx context.Context, link string, progress func(string)) (string, error) {
			return "video.part", nil
		},
	}
	h := newTestHandler(t, dl, &mockFileStore{})
	h.Link(context.Background(), b, &models.Update{
		Message: &models.Message{Text: "https://youtube.com/watch?v=test"},
	})
	if len(edits) < 1 || edits[len(edits)-1] != "Не удалось скачать видео 😢" {
		t.Errorf("expected 'Не удалось скачать видео', got %v", edits)
	}
}

func TestHandler_Link_Success(t *testing.T) {
	var edits []string
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			return &models.Message{ID: 1, Chat: models.Chat{ID: 100}}, nil
		},
		editMessageTextFn: func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
			edits = append(edits, params.Text)
			return &models.Message{}, nil
		},
		sendVideoFn: func(ctx context.Context, params *bot.SendVideoParams) (*models.Message, error) {
			return &models.Message{}, nil
		},
	}
	dl := &mockDownloader{
		downloadFn: func(ctx context.Context, link string, progress func(string)) (string, error) {
			return "result.mp4", nil
		},
	}
	h := newTestHandler(t, dl, &mockFileStore{})
	h.Link(context.Background(), b, &models.Update{
		Message: &models.Message{Text: "https://youtube.com/watch?v=test"},
	})
	if len(edits) < 1 || edits[len(edits)-1] != "🎉" {
		t.Errorf("expected success flow, got edits %v", edits)
	}
}

func TestHandler_sendVideo_EmptyFileName(t *testing.T) {
	b := &mockBot{
		editMessageTextFn: func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
			if params.Text != "Не удалось скачать видео 😢" {
				t.Errorf("got text %q", params.Text)
			}
			return &models.Message{}, nil
		},
	}
	h := newTestHandler(t, &mockDownloader{}, &mockFileStore{})
	h.sendVideo(context.Background(), b, 100, 1, "", "", time.Time{})
}

func TestHandler_sendVideo_PartFile(t *testing.T) {
	b := &mockBot{
		editMessageTextFn: func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
			if params.Text != "Не удалось скачать видео 😢" {
				t.Errorf("got text %q", params.Text)
			}
			return &models.Message{}, nil
		},
	}
	h := newTestHandler(t, &mockDownloader{}, &mockFileStore{})
	h.sendVideo(context.Background(), b, 100, 1, "video.part", "", time.Time{})
}

func TestHandler_sendVideo_FileNotFound(t *testing.T) {
	b := &mockBot{
		editMessageTextFn: func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
			if params.Text != "Не удалось открыть видео 😢" {
				t.Errorf("got text %q", params.Text)
			}
			return &models.Message{}, nil
		},
	}
	fs := &mockFileStore{
		openFn: func(name string) (io.ReadCloser, error) {
			return nil, errors.New("not found")
		},
	}
	h := newTestHandler(t, &mockDownloader{}, fs)
	h.sendVideo(context.Background(), b, 100, 1, "video.mp4", "", time.Time{})
}

func TestHandler_sendVideo_SendFails(t *testing.T) {
	var edits []string
	b := &mockBot{
		editMessageTextFn: func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
			edits = append(edits, params.Text)
			return &models.Message{}, nil
		},
		sendVideoFn: func(ctx context.Context, params *bot.SendVideoParams) (*models.Message, error) {
			if len(edits) == 0 || edits[len(edits)-1] != "Отправляю..." {
				t.Error("expected 'Отправляю...' edit before SendVideo")
			}
			return nil, errors.New("send error")
		},
	}
	fs := &mockFileStore{}
	h := newTestHandler(t, &mockDownloader{}, fs)
	h.sendVideo(context.Background(), b, 100, 1, "video.mp4", "", time.Time{})

	if len(edits) < 2 || edits[len(edits)-1] != "Не удалось отправить видео 😢" {
		t.Errorf("expected error edit, got %v", edits)
	}
}

func TestHandler_sendVideo_Success(t *testing.T) {
	var edits []string
	var removed bool
	b := &mockBot{
		editMessageTextFn: func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
			edits = append(edits, params.Text)
			return &models.Message{}, nil
		},
		sendVideoFn: func(ctx context.Context, params *bot.SendVideoParams) (*models.Message, error) {
			return &models.Message{}, nil
		},
	}
	fs := &mockFileStore{
		openFn: func(name string) (io.ReadCloser, error) {
			if name != "video.mp4" {
				t.Errorf("got name %q, want %q", name, "video.mp4")
			}
			return io.NopCloser(strings.NewReader("data")), nil
		},
		removeFn: func(name string) error {
			removed = true
			if name != "video.mp4" {
				t.Errorf("got name %q for remove", name)
			}
			return nil
		},
	}
	h := newTestHandler(t, &mockDownloader{}, fs)
	h.sendVideo(context.Background(), b, 100, 1, "video.mp4", "", time.Time{})

	if len(edits) < 2 || edits[len(edits)-1] != "🎉" {
		t.Errorf("expected 🎉 as last edit, got %v", edits)
	}
	if !removed {
		t.Error("expected file to be removed")
	}
}

func TestHandler_sendVideo_RemoveError(t *testing.T) {
	b := &mockBot{
		editMessageTextFn: func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
			return &models.Message{}, nil
		},
		sendVideoFn: func(ctx context.Context, params *bot.SendVideoParams) (*models.Message, error) {
			return &models.Message{}, nil
		},
	}
	fs := &mockFileStore{
		openFn: func(name string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("data")), nil
		},
		removeFn: func(name string) error {
			return errors.New("remove error")
		},
	}
	h := newTestHandler(t, &mockDownloader{}, fs)
	h.sendVideo(context.Background(), b, 100, 1, "video.mp4", "", time.Time{})
}

func TestHandler_downloadVideoByLink_Success(t *testing.T) {
	b := &mockBot{}
	dl := &mockDownloader{
		downloadFn: func(ctx context.Context, link string, progress func(string)) (string, error) {
			return "result.mp4", nil
		},
	}
	h := newTestHandler(t, dl, &mockFileStore{})
	fn := h.downloadVideoByLink(context.Background(), b, 100, 1, "https://example.com/video")
	if fn != "result.mp4" {
		t.Errorf("got %q, want %q", fn, "result.mp4")
	}
}

func TestHandler_downloadVideoByLink_Error(t *testing.T) {
	var errorEdit string
	b := &mockBot{
		editMessageTextFn: func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
			errorEdit = params.Text
			return &models.Message{}, nil
		},
	}
	dl := &mockDownloader{
		downloadFn: func(ctx context.Context, link string, progress func(string)) (string, error) {
			return "", errors.New("script failed")
		},
	}
	h := newTestHandler(t, dl, &mockFileStore{})
	fn := h.downloadVideoByLink(context.Background(), b, 100, 1, "https://example.com/video")
	if fn != "" {
		t.Errorf("got %q, want empty", fn)
	}
	if errorEdit != "Что-то пошло не так" {
		t.Errorf("got edit %q", errorEdit)
	}
}

func TestHandler_Stats_NilMessage(t *testing.T) {
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			t.Error("unexpected call")
			return nil, nil
		},
	}
	h := newTestHandler(t, &mockDownloader{}, &mockFileStore{})
	h.Stats(context.Background(), b, &models.Update{})
}

func TestHandler_Stats_NilFrom(t *testing.T) {
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			t.Error("unexpected call")
			return nil, nil
		},
	}
	h := newTestHandler(t, &mockDownloader{}, &mockFileStore{})
	h.Stats(context.Background(), b, &models.Update{
		Message: &models.Message{},
	})
}

func TestHandler_Stats_AccessDenied(t *testing.T) {
	var sentText string
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			sentText = params.Text
			return &models.Message{}, nil
		},
	}
	h := newTestHandler(t, &mockDownloader{}, &mockFileStore{})
	h.Stats(context.Background(), b, &models.Update{
		Message: &models.Message{
			From: &models.User{ID: 999},
			Chat: models.Chat{ID: 1},
		},
	})
	if sentText != "Access denied." {
		t.Errorf("got %q, want 'Access denied.'", sentText)
	}
}

func TestHandler_Stats_Success(t *testing.T) {
	var sentText string
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			sentText = params.Text
			return &models.Message{}, nil
		},
	}
	h := newTestHandler(t, &mockDownloader{}, &mockFileStore{})
	h.adminIDs = []int64{42}
	h.Stats(context.Background(), b, &models.Update{
		Message: &models.Message{
			From: &models.User{ID: 42},
			Chat: models.Chat{ID: 1},
		},
	})
	if !strings.Contains(sentText, "Статистика бота") {
		t.Errorf("expected report, got %q", sentText)
	}
}

func TestExtractDomain_Valid(t *testing.T) {
	got := extractDomain("https://www.youtube.com/watch?v=test")
	if got != "www.youtube.com" {
		t.Errorf("got %q, want %q", got, "www.youtube.com")
	}
}

func TestExtractDomain_Invalid(t *testing.T) {
	got := extractDomain("://invalid")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestHandler_sendVideo_Success_WithFileSize(t *testing.T) {
	var edits []string
	b := &mockBot{
		editMessageTextFn: func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
			edits = append(edits, params.Text)
			return &models.Message{}, nil
		},
		sendVideoFn: func(ctx context.Context, params *bot.SendVideoParams) (*models.Message, error) {
			return &models.Message{}, nil
		},
	}
	fs := &mockFileStore{
		openFn: func(name string) (io.ReadCloser, error) {
			if name != "video.mp4" {
				t.Errorf("got name %q, want %q", name, "video.mp4")
			}
			return &statReadCloser{ReadCloser: io.NopCloser(strings.NewReader("data")), size: 42}, nil
		},
		removeFn: func(name string) error { return nil },
	}
	h := newTestHandler(t, &mockDownloader{}, fs)
	h.sendVideo(context.Background(), b, 100, 1, "video.mp4", "", time.Time{})

	if len(edits) < 2 || edits[len(edits)-1] != "🎉" {
		t.Errorf("expected 🎉 as last edit, got %v", edits)
	}
}

func TestHandler_downloadVideoByLink_Progress(t *testing.T) {
	var progressEdits []string
	b := &mockBot{
		editMessageTextFn: func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
			progressEdits = append(progressEdits, params.Text)
			return &models.Message{}, nil
		},
	}
	dl := &mockDownloader{
		downloadFn: func(ctx context.Context, link string, progress func(string)) (string, error) {
			progress("Загружаю...")
			progress("Конвертирую...")
			return "done.mp4", nil
		},
	}
	h := newTestHandler(t, dl, &mockFileStore{})
	fn := h.downloadVideoByLink(context.Background(), b, 100, 1, "https://example.com/video")
	if fn != "done.mp4" {
		t.Errorf("got %q, want %q", fn, "done.mp4")
	}
	if len(progressEdits) != 2 || progressEdits[0] != "Загружаю..." || progressEdits[1] != "Конвертирую..." {
		t.Errorf("got progress edits %v", progressEdits)
	}
}
