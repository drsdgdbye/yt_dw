package downloader

import (
	"bufio"
	"context"
	"log/slog"
	"os/exec"
	"strings"
)

// Downloader — интерфейс для скачивания видео по ссылке.
type Downloader interface {
	// Download скачивает видео, возвращает имя файла.
	// progress вызывается с промежуточными статусами.
	Download(ctx context.Context, link string, progress func(string)) (string, error)
}

// BashDownloader реализует Downloader через bash-скрипт с yt-dlp.
type BashDownloader struct {
	scriptPath string
}

// New создаёт BashDownloader с путём к скрипту.
func New(scriptPath string) *BashDownloader {
	return &BashDownloader{scriptPath: scriptPath}
}

// Download запускает bash-скрипт, парсит stdout на [INFO] и [ID].
func (d *BashDownloader) Download(ctx context.Context, link string, progress func(string)) (string, error) {
	cmd := exec.Command("bash", d.scriptPath, link)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	var fileName string

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			s := scanner.Text()
			if strings.HasPrefix(s, "[INFO]") {
				info := strings.Replace(s, "[INFO]: ", "", 1)
				if progress != nil {
					progress(info)
				}
			} else if strings.HasPrefix(s, "[ID]") {
				fn := strings.Replace(s, "[ID]: ", "", 1)
				fileName = fn
			} else {
				slog.DebugContext(ctx, s)
			}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			slog.ErrorContext(ctx, scanner.Text())
		}
	}()

	if wErr := cmd.Wait(); wErr != nil {
		return "", wErr
	}

	return fileName, nil
}
