package telegram

import (
	"context"
	"io"
	"strings"

	"yt_dw/internal/downloader"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type mockBot struct {
	sendMessageFn    func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
	editMessageTextFn func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error)
	sendVideoFn      func(ctx context.Context, params *bot.SendVideoParams) (*models.Message, error)
	sendPhotoFn      func(ctx context.Context, params *bot.SendPhotoParams) (*models.Message, error)
	answerCallbackFn func(ctx context.Context, params *bot.AnswerCallbackQueryParams) (bool, error)
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

func (m *mockBot) SendPhoto(ctx context.Context, params *bot.SendPhotoParams) (*models.Message, error) {
	if m.sendPhotoFn != nil {
		return m.sendPhotoFn(ctx, params)
	}
	return &models.Message{}, nil
}

func (m *mockBot) AnswerCallbackQuery(ctx context.Context, params *bot.AnswerCallbackQueryParams) (bool, error) {
	if m.answerCallbackFn != nil {
		return m.answerCallbackFn(ctx, params)
	}
	return true, nil
}

type mockDownloader struct {
	downloadFn     func(ctx context.Context, link string, progress func(string)) (string, error)
	listFn         func(ctx context.Context, link string) ([]downloader.Media, error)
	downloadItemFn func(ctx context.Context, link string, index int, progress func(string)) (string, error)
}

func (m *mockDownloader) Download(ctx context.Context, link string, progress func(string)) (string, error) {
	if m.downloadFn != nil {
		return m.downloadFn(ctx, link, progress)
	}
	return "video.mp4", nil
}

func (m *mockDownloader) List(ctx context.Context, link string) ([]downloader.Media, error) {
	if m.listFn != nil {
		return m.listFn(ctx, link)
	}
	return []downloader.Media{{Index: 1, Kind: "video"}}, nil
}

func (m *mockDownloader) DownloadItem(ctx context.Context, link string, index int, progress func(string)) (string, error) {
	if m.downloadItemFn != nil {
		return m.downloadItemFn(ctx, link, index, progress)
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
