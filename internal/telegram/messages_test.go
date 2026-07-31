package telegram

import (
	"context"
	"errors"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func TestSendMessage_Success(t *testing.T) {
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			if params.ChatID != int64(100) {
				t.Errorf("got chatID %v, want 100", params.ChatID)
			}
			if params.Text != "hello" {
				t.Errorf("got text %q, want %q", params.Text, "hello")
			}
			return &models.Message{ID: 5, Chat: models.Chat{ID: 100}}, nil
		},
	}

	SendMessage(context.Background(), b, 100, "hello")
}

func TestSendMessage_BotError(t *testing.T) {
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			return nil, errors.New("bot error")
		},
	}

	SendMessage(context.Background(), b, 100, "hello")
}

func TestSendMessageWith_Success(t *testing.T) {
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			return &models.Message{ID: 10, Chat: models.Chat{ID: 200}}, nil
		},
	}

	msg, err := SendMessageWith(context.Background(), b, 200, "test")
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID != 10 {
		t.Errorf("got msg.ID %d, want 10", msg.ID)
	}
}

func TestSendMessageWith_Error(t *testing.T) {
	b := &mockBot{
		sendMessageFn: func(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
			return nil, errors.New("fail")
		},
	}

	_, err := SendMessageWith(context.Background(), b, 200, "test")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEditMessage_Success(t *testing.T) {
	var called bool
	b := &mockBot{
		editMessageTextFn: func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
			called = true
			if params.ChatID != int64(300) {
				t.Errorf("got chatID %v, want 300", params.ChatID)
			}
			if params.MessageID != 7 {
				t.Errorf("got msgID %d, want 7", params.MessageID)
			}
			if params.Text != "updated" {
				t.Errorf("got text %q, want %q", params.Text, "updated")
			}
			return &models.Message{}, nil
		},
	}

	EditMessage(context.Background(), b, 300, 7, "updated")
	if !called {
		t.Fatal("EditMessageText was not called")
	}
}

func TestEditMessage_Error(t *testing.T) {
	b := &mockBot{
		editMessageTextFn: func(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error) {
			return nil, errors.New("edit error")
		},
	}

	EditMessage(context.Background(), b, 300, 7, "updated")
}
