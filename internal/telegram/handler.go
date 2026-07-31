package telegram

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"yt_dw/internal/downloader"
	"yt_dw/internal/filestore"
	"yt_dw/internal/stats"
	"yt_dw/internal/validator"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Handler — основной обработчик команд и сообщений бота.
type Handler struct {
	downloader downloader.Downloader
	store      filestore.FileStore
	stats      *stats.Stats
	adminIDs   []int64
}

// NewHandler создаёт Handler с переданными зависимостями.
func NewHandler(d downloader.Downloader, s filestore.FileStore, st *stats.Stats, adminIDs []int64) *Handler {
	return &Handler{downloader: d, store: s, stats: st, adminIDs: adminIDs}
}

// Start — обработчик /start. Отвечает приветствием и считает новый чат.
func (h *Handler) Start(ctx context.Context, b BotClient, update *models.Update) {
	if update.Message == nil {
		return
	}

	h.stats.IncrementNewChats()
	SendMessage(ctx, b, update.Message.Chat.ID, "Стартуем 🚀")
}

// Link — обработчик ссылок. Валидирует, скачивает, отправляет видео, собирает статистику.
func (h *Handler) Link(ctx context.Context, b BotClient, update *models.Update) {
	if update.Message == nil {
		return
	}

	startTime := time.Now()
	chatID := update.Message.Chat.ID

	h.stats.IncrementProcessed()

	m, sendErr := SendMessageWith(ctx, b, chatID, "Проверяю ссылку...")

	if sendErr != nil || m == nil {
		if sendErr != nil {
			slog.ErrorContext(ctx, "send message", "error", sendErr, "chatID", chatID)
		}
		return
	}

	link := update.Message.Text
	msgID := m.ID

	domain := extractDomain(link)

	if !validator.IsValidURL(link) {
		EditMessage(ctx, b, chatID, msgID, "Ссылка не валидна 😔")
		slog.ErrorContext(ctx, "link validation failed", "link", link, "chatID", chatID)
		h.stats.IncrementFailed(chatID, "invalid_url")
		return
	}

	fileName := h.downloadVideoByLink(ctx, b, chatID, msgID, link)
	if fileName == "" {
		h.stats.IncrementFailed(chatID, "download_error")
		return
	}

	h.sendVideo(ctx, b, chatID, msgID, fileName, domain, startTime)
}

// Stats — обработчик /stats. Выдаёт отчёт статистики только администраторам.
func (h *Handler) Stats(ctx context.Context, b BotClient, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	if !h.isAdmin(update.Message.From.ID) {
		SendMessage(ctx, b, update.Message.Chat.ID, "Access denied.")
		return
	}

	report := h.stats.Report()
	SendMessage(ctx, b, update.Message.Chat.ID, report)
}

// isAdmin проверяет, есть ли userID в списке администраторов.
func (h *Handler) isAdmin(userID int64) bool {
	for _, id := range h.adminIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// extractDomain извлекает домен из URL.
func extractDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// Default — обработчик для обновлений, не подходящих под другие хендлеры.
func (h *Handler) Default(ctx context.Context, b BotClient, update *models.Update) {
	if update.Message == nil {
		return
	}

	SendMessage(ctx, b, update.Message.Chat.ID, "Отправь мне полную ссылку на видео в формате: `https://link.to/your/video`")
}

// downloadVideoByLink запускает скачивание и возвращает имя файла.
func (h *Handler) downloadVideoByLink(ctx context.Context, b BotClient, chatID int64, msgID int, link string) string {
	fileName, err := h.downloader.Download(ctx, link, func(info string) {
		EditMessage(ctx, b, chatID, msgID, info)
	})

	if err != nil {
		EditMessage(ctx, b, chatID, msgID, "Что-то пошло не так")
		slog.ErrorContext(ctx, "starting script", "error", err, "chatID", chatID)
		return ""
	}

	return fileName
}

// sendVideo открывает файл, отправляет видео пользователю и обновляет статистику.
func (h *Handler) sendVideo(ctx context.Context, b BotClient, chatID int64, msgID int, fileName, domain string, startTime time.Time) {
	if fileName == "" || strings.HasSuffix(fileName, ".part") {
		EditMessage(ctx, b, chatID, msgID, "Не удалось скачать видео 😢")
		slog.ErrorContext(ctx, "failing download video", "fileName", fileName)
		h.stats.IncrementFailed(chatID, "download_error")
		return
	}

	fn := strings.Split(fileName, ".")[0] + ".mp4"

	file, err := h.store.Open(fn)
	if err != nil {
		EditMessage(ctx, b, chatID, msgID, "Не удалось открыть видео 😢")
		slog.ErrorContext(ctx, "opening file", "error", err, "fileName", fileName)
		h.stats.IncrementFailed(chatID, "open_error")
		return
	}
	defer file.Close()

	var fileSize int64
	if f, ok := file.(interface{ Stat() (os.FileInfo, error) }); ok {
		if fi, err := f.Stat(); err == nil {
			fileSize = fi.Size()
		}
	}

	params := &bot.SendVideoParams{
		ChatID: chatID,
		Video:  &models.InputFileUpload{Filename: fn, Data: file},
	}

	EditMessage(ctx, b, chatID, msgID, "Отправляю...")

	_, svErr := b.SendVideo(ctx, params)
	if svErr != nil {
		EditMessage(ctx, b, chatID, msgID, "Не удалось отправить видео 😢")
		slog.ErrorContext(ctx, "sending video", "error", svErr, "chatID", chatID)
		h.stats.IncrementFailed(chatID, "send_error")
		return
	}

	EditMessage(ctx, b, chatID, msgID, "🎉")

	procTimeMs := time.Since(startTime).Milliseconds()
	h.stats.IncrementSuccess(chatID, domain, fileSize, procTimeMs)

	if err := h.store.Remove(fn); err != nil {
		slog.ErrorContext(ctx, "removing file", "error", err, "fileName", fn)
	}
}
