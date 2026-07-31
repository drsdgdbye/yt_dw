package config

import (
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Telegram TelegramConfig `yaml:"telegram"`
	Log      LogConfig      `yaml:"log"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type TelegramConfig struct {
	Token    string  `yaml:"token"`
	AdminIDs []int64 `yaml:"admin_ids"`
}

func NewConfig(cfgName string) *Config {
	var cfg Config
	file, err := os.Open(cfgName)
	if err != nil {
		log.Fatalf("open config file %s error: %v", cfgName, err)
	}
	defer func(file *os.File) {
		if err := file.Close(); err != nil {
			slog.Error(err.Error())
		}
	}(file)
	decoder := yaml.NewDecoder(file)
	if err = decoder.Decode(&cfg); err != nil {
		log.Fatalf("error decoding yaml: %v", err)
	}

	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "text"
	}

	if envToken := os.Getenv("TELEGRAM_TOKEN"); envToken != "" {
		cfg.Telegram.Token = envToken
	}
	if envAdminIDs := os.Getenv("TELEGRAM_ADMIN_IDS"); envAdminIDs != "" {
		parts := strings.Split(envAdminIDs, ",")
		ids := make([]int64, 0, len(parts))
		for _, p := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil {
			slog.Error("invalid admin ID in TELEGRAM_ADMIN_IDS", "id", p)
			continue
		}
		ids = append(ids, id)
		}
		cfg.Telegram.AdminIDs = ids
	}

	return &cfg
}
