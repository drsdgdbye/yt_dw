package telegram

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// EditMessage изменяет текст существующего сообщения.
func EditMessage(ctx context.Context, b BotClient, chatID int64, msgID int, text string) {
	_, editErr := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: msgID,
		Text:      text,
	})

	if editErr != nil {
		slog.ErrorContext(ctx, "editing message", "error", editErr, "chatID", chatID)
		return
	}
}

// SendMessageWith отправляет сообщение и возвращает ответ API.
func SendMessageWith(ctx context.Context, b BotClient, chatID int64, text string) (*models.Message, error) {
	m, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})

	return m, err
}

// SendMessage отправляет сообщение, логируя ошибку.
func SendMessage(ctx context.Context, b BotClient, chatID int64, text string) {
	if _, err := SendMessageWith(ctx, b, chatID, text); err != nil {
		slog.ErrorContext(ctx, "sending message", "error", err, "chatID", chatID)
	}
}
