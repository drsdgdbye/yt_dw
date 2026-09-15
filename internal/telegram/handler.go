package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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

	h.stats.TrackChat(chatID, senderUsername(update))
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

	if code, ok := instagramPostCode(link); ok {
		h.handlePost(ctx, b, chatID, msgID, link, code, domain, startTime)
		return
	}

	fileName, dlErr := h.downloadByLink(ctx, b, chatID, msgID, link, 0)
	if dlErr != nil {
		h.stats.IncrementFailed(chatID, downloadErrorCode(dlErr))
		return
	}
	if fileName == "" {
		h.stats.IncrementFailed(chatID, "download_error")
		return
	}

	h.sendMedia(ctx, b, chatID, msgID, fileName, domain, startTime)
}

// handlePost обрабатывает пост Instagram: один элемент отправляет сразу, из нескольких предлагает выбрать.
func (h *Handler) handlePost(ctx context.Context, b BotClient, chatID int64, msgID int, link, code, domain string, startTime time.Time) {
	items, err := h.downloader.List(ctx, link)
	if err != nil {
		EditMessage(ctx, b, chatID, msgID, downloadErrorMessage(err))
		slog.ErrorContext(ctx, "listing media", "error", err, "chatID", chatID)
		h.stats.IncrementFailed(chatID, downloadErrorCode(err))
		return
	}

	if len(items) == 0 {
		EditMessage(ctx, b, chatID, msgID, "В посте нет медиа 😔")
		h.stats.IncrementFailed(chatID, "media_list_error")
		return
	}

	if len(items) == 1 {
		h.sendItem(ctx, b, chatID, msgID, link, 1, domain, startTime)
		return
	}

	h.sendPicker(ctx, b, chatID, msgID, code, items)
}

// sendItem скачивает элемент поста и отправляет его.
func (h *Handler) sendItem(ctx context.Context, b BotClient, chatID int64, msgID int, link string, index int, domain string, startTime time.Time) {
	fileName, err := h.downloadByLink(ctx, b, chatID, msgID, link, index)
	if err != nil {
		h.stats.IncrementFailed(chatID, downloadErrorCode(err))
		return
	}
	if fileName == "" {
		h.stats.IncrementFailed(chatID, "download_error")
		return
	}

	h.sendMedia(ctx, b, chatID, msgID, fileName, domain, startTime)
}

// sendPicker показывает инлайн-кнопки для выбора элемента поста.
func (h *Handler) sendPicker(ctx context.Context, b BotClient, chatID int64, msgID int, code string, items []downloader.Media) {
	const perRow = 5

	buttons := make([]models.InlineKeyboardButton, 0, len(items))
	for _, item := range items {
		buttons = append(buttons, models.InlineKeyboardButton{
			Text:         fmt.Sprintf("%d · %s", item.Index, mediaLabel(item.Kind)),
			CallbackData: fmt.Sprintf("ig:%s:%d", code, item.Index),
		})
	}

	rows := make([][]models.InlineKeyboardButton, 0, (len(buttons)+perRow-1)/perRow)
	for len(buttons) > 0 {
		n := perRow
		if len(buttons) < n {
			n = len(buttons)
		}
		rows = append(rows, buttons[:n])
		buttons = buttons[n:]
	}

	EditMessageWithKeyboard(ctx, b, chatID, msgID,
		fmt.Sprintf("В посте %d медиа. Выбери, что отправить:", len(items)),
		&models.InlineKeyboardMarkup{InlineKeyboard: rows})
}

// PickMedia — обработчик выбора элемента поста по инлайн-кнопке.
func (h *Handler) PickMedia(ctx context.Context, b BotClient, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	cq := update.CallbackQuery
	AnswerCallback(ctx, b, cq.ID)

	var chatID int64
	var msgID int
	switch {
	case cq.Message.Message != nil:
		chatID = cq.Message.Message.Chat.ID
		msgID = cq.Message.Message.ID
	case cq.Message.InaccessibleMessage != nil:
		chatID = cq.Message.InaccessibleMessage.Chat.ID
		msgID = cq.Message.InaccessibleMessage.MessageID
	default:
		return
	}

	code, index, ok := parsePickData(cq.Data)
	if !ok {
		return
	}

	link := "https://www.instagram.com/p/" + code + "/"

	h.stats.TrackChat(chatID, cq.From.Username)
	ClearKeyboard(ctx, b, chatID, msgID, "Скачиваю...")

	h.sendItem(ctx, b, chatID, msgID, link, index, extractDomain(link), time.Now())
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

// senderUsername возвращает username отправителя или пустую строку.
func senderUsername(update *models.Update) string {
	if update.Message == nil || update.Message.From == nil {
		return ""
	}
	return update.Message.From.Username
}

// Default — обработчик для обновлений, не подходящих под другие хендлеры.
func (h *Handler) Default(ctx context.Context, b BotClient, update *models.Update) {
	if update.Message == nil {
		return
	}

	SendMessage(ctx, b, update.Message.Chat.ID, "Отправь мне полную ссылку на видео в формате: `https://link.to/your/video`")
}

// downloadByLink запускает скачивание видео (item == 0) или элемента поста.
func (h *Handler) downloadByLink(ctx context.Context, b BotClient, chatID int64, msgID int, link string, item int) (string, error) {
	progress := func(info string) {
		EditMessage(ctx, b, chatID, msgID, info)
	}

	var (
		fileName string
		err      error
	)
	if item > 0 {
		fileName, err = h.downloader.DownloadItem(ctx, link, item, progress)
	} else {
		fileName, err = h.downloader.Download(ctx, link, progress)
	}

	if err != nil {
		EditMessage(ctx, b, chatID, msgID, downloadErrorMessage(err))
		slog.ErrorContext(ctx, "starting script", "error", err, "chatID", chatID)
		return "", err
	}

	return fileName, nil
}

// downloadErrorMessage возвращает текст ошибки для чата.
func downloadErrorMessage(err error) string {
	var scriptErr *downloader.ScriptError
	if errors.As(err, &scriptErr) && scriptErr.Reason != "" {
		return scriptErr.Reason
	}
	return "Что-то пошло не так"
}

// downloadErrorCode возвращает категорию ошибки для статистики.
func downloadErrorCode(err error) string {
	var scriptErr *downloader.ScriptError
	if errors.As(err, &scriptErr) && scriptErr.Code != "" {
		return scriptErr.Code
	}
	return "download_error"
}

// sendMedia открывает файл, отправляет фото или видео и обновляет статистику.
func (h *Handler) sendMedia(ctx context.Context, b BotClient, chatID int64, msgID int, fileName, domain string, startTime time.Time) {
	if fileName == "" || strings.HasSuffix(fileName, ".part") {
		EditMessage(ctx, b, chatID, msgID, "Не удалось скачать медиа 😢")
		slog.ErrorContext(ctx, "failing download media", "fileName", fileName)
		h.stats.IncrementFailed(chatID, "download_error")
		return
	}

	photo := isImageFile(fileName)
	fn := fileName
	kind := "фото"
	if !photo {
		fn = strings.Split(fileName, ".")[0] + ".mp4"
		kind = "видео"
	}

	file, err := h.store.Open(fn)
	if err != nil {
		EditMessage(ctx, b, chatID, msgID, "Не удалось открыть "+kind+" 😢")
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

	EditMessage(ctx, b, chatID, msgID, "Отправляю...")

	var svErr error
	if photo {
		_, svErr = b.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID: chatID,
			Photo:  &models.InputFileUpload{Filename: fn, Data: file},
		})
	} else {
		_, svErr = b.SendVideo(ctx, &bot.SendVideoParams{
			ChatID: chatID,
			Video:  &models.InputFileUpload{Filename: fn, Data: file},
		})
	}

	if svErr != nil {
		EditMessage(ctx, b, chatID, msgID, "Не удалось отправить "+kind+" 😢")
		slog.ErrorContext(ctx, "sending media", "error", svErr, "chatID", chatID)
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

// instagramPostCode возвращает shortcode поста Instagram (/p/ или /tv/).
func instagramPostCode(rawURL string) (string, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}

	host := strings.ToLower(parsed.Hostname())
	if host != "instagram.com" && !strings.HasSuffix(host, ".instagram.com") {
		return "", false
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "p" || parts[i] == "tv" {
			return parts[i+1], true
		}
	}

	return "", false
}

// parsePickData разбирает callback data вида "ig:<shortcode>:<номер>".
func parsePickData(data string) (string, int, bool) {
	parts := strings.Split(data, ":")
	if len(parts) != 3 || parts[0] != "ig" {
		return "", 0, false
	}

	index, err := strconv.Atoi(parts[2])
	if err != nil || index < 1 {
		return "", 0, false
	}

	return parts[1], index, true
}

// mediaLabel возвращает подпись типа медиа для кнопки.
func mediaLabel(kind string) string {
	if kind == "photo" {
		return "фото"
	}
	return "видео"
}

// isImageFile проверяет, что файл — картинка.
func isImageFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp":
		return true
	}
	return false
}
