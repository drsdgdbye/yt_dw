package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// BotClient — минимальный интерфейс Telegram Bot API для тестирования.
type BotClient interface {
	SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
	EditMessageText(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error)
	SendVideo(ctx context.Context, params *bot.SendVideoParams) (*models.Message, error)
}
