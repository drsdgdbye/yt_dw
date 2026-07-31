package telegram

import (
	"context"
	"io"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type mockBot struct {
	sendMessageFn    func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
	editMessageTextFn func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error)
	sendVideoFn      func(ctx context.Context, params *bot.SendVideoParams) (*models.Message, error)
}

func (m *mockBot) SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	if m.sendMessageFn != nil {
		return m.sendMessageFn(ctx, params)
	}
	return &models.Message{ID: 1, Chat: models.Chat{ID: 123}}, nil
}

func (m *mockBot) EditMessageText(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
	if m.editMessageTextFn != nil {
		return m.editMessageTextFn(ctx, params)
	}
	return &models.Message{}, nil
}

func (m *mockBot) SendVideo(ctx context.Context, params *bot.SendVideoParams) (*models.Message, error) {
	if m.sendVideoFn != nil {
		return m.sendVideoFn(ctx, params)
	}
	return &models.Message{}, nil
}

type mockDownloader struct {
	downloadFn func(ctx context.Context, link string, progress func(string)) (string, error)
}

func (m *mockDownloader) Download(ctx context.Context, link string, progress func(string)) (string, error) {
	if m.downloadFn != nil {
		return m.downloadFn(ctx, link, progress)
	}
	return "video.mp4", nil
}

type mockFileStore struct {
	openFn   func(name string) (io.ReadCloser, error)
	removeFn func(name string) error
}

func (m *mockFileStore) Open(name string) (io.ReadCloser, error) {
	if m.openFn != nil {
		return m.openFn(name)
	}
	return io.NopCloser(strings.NewReader("data")), nil
}

func (m *mockFileStore) Remove(name string) error {
	if m.removeFn != nil {
		return m.removeFn(name)
	}
	return nil
}
