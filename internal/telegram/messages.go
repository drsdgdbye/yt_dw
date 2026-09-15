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

// EditMessageWithKeyboard изменяет текст сообщения и заменяет инлайн-клавиатуру.
func EditMessageWithKeyboard(ctx context.Context, b BotClient, chatID int64, msgID int, text string, markup *models.InlineKeyboardMarkup) {
	_, editErr := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   msgID,
		Text:        text,
		ReplyMarkup: markup,
	})

	if editErr != nil {
		slog.ErrorContext(ctx, "editing message", "error", editErr, "chatID", chatID)
	}
}

// ClearKeyboard убирает инлайн-клавиатуру, оставляя новый текст.
func ClearKeyboard(ctx context.Context, b BotClient, chatID int64, msgID int, text string) {
	EditMessageWithKeyboard(ctx, b, chatID, msgID, text, &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{},
	})
}

// AnswerCallback подтверждает callback-запрос, убирая индикатор загрузки.
func AnswerCallback(ctx context.Context, b BotClient, callbackID string) {
	if _, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
	}); err != nil {
		slog.ErrorContext(ctx, "answering callback", "error", err)
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
