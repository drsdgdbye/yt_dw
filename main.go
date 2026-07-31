package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"yt_dw/config"
	"yt_dw/internal/downloader"
	"yt_dw/internal/filestore"
	"yt_dw/internal/logger"
	"yt_dw/internal/stats"
	"yt_dw/internal/telegram"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// main — точка входа: конфиг, логгер, зависимости, запуск long-polling.
func main() {
	const (
		pollTimeout       = 120 * time.Second
		httpClientTimeout = 130 * time.Second
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer cancel()

	cfg := config.NewConfig("application.yaml")
	logger.Init(cfg.Log.Level, cfg.Log.Format)

	dw := downloader.New("script/yt_dw.sh")
	fs := filestore.New("/var/tmp/yt_dw/")
	st := stats.New("/var/tmp/yt_dw/stats.json")
	defer st.Save()
	go st.StartPeriodicSave(ctx, 30*time.Second)
	h := telegram.NewHandler(dw, fs, st, cfg.Telegram.AdminIDs)

	opts := []bot.Option{
		bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
			h.Default(ctx, b, update)
		}),
		bot.WithMessageTextHandler("/start", bot.MatchTypeExact, func(ctx context.Context, b *bot.Bot, update *models.Update) {
			h.Start(ctx, b, update)
		}),
		bot.WithMessageTextHandler("/stats", bot.MatchTypeExact, func(ctx context.Context, b *bot.Bot, update *models.Update) {
			h.Stats(ctx, b, update)
		}),
		bot.WithMessageTextHandler("https://", bot.MatchTypePrefix, func(ctx context.Context, b *bot.Bot, update *models.Update) {
			h.Link(ctx, b, update)
		}),
		bot.WithHTTPClient(pollTimeout, &http.Client{
			Timeout: httpClientTimeout,
		}),
	}

	b, err := bot.New(cfg.Telegram.Token, opts...)
	if nil != err {
		panic(err)
	}

	b.Start(ctx)
}
