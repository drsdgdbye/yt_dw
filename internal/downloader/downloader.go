package downloader

import (
	"bufio"
	"context"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Media — элемент поста: номер (1-based) и тип (photo или video).
type Media struct {
	Index int
	Kind  string
}

// Downloader — интерфейс для скачивания видео и элементов постов.
type Downloader interface {
	// Download скачивает видео, возвращает имя файла.
	// progress вызывается с промежуточными статусами.
	Download(ctx context.Context, link string, progress func(string)) (string, error)
	// List возвращает элементы поста (фото и видео).
	List(ctx context.Context, link string) ([]Media, error)
	// DownloadItem скачивает элемент поста по его номеру (1-based).
	DownloadItem(ctx context.Context, link string, index int, progress func(string)) (string, error)
}

// ScriptError — ошибка bash-скрипта: код для статистики и текст для пользователя.
type ScriptError struct {
	Code   string
	Reason string
	Err    error
}

// Error возвращает текстовое представление ошибки.
func (e *ScriptError) Error() string {
	if e.Code == "" {
		return e.Reason
	}
	return e.Code + ": " + e.Reason
}

// Unwrap даёт доступ к исходной ошибке процесса.
func (e *ScriptError) Unwrap() error {
	return e.Err
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
	res, err := d.run(ctx, []string{link}, progress)
	if err != nil {
		return "", err
	}

	return res.fileName, nil
}

// List возвращает элементы поста, которые отдаёт скрипт.
func (d *BashDownloader) List(ctx context.Context, link string) ([]Media, error) {
	res, err := d.run(ctx, []string{"--list", link}, nil)
	if err != nil {
		return nil, err
	}

	return res.media, nil
}

// DownloadItem скачивает элемент поста по его номеру (1-based).
func (d *BashDownloader) DownloadItem(ctx context.Context, link string, index int, progress func(string)) (string, error) {
	res, err := d.run(ctx, []string{"--item", strconv.Itoa(index), link}, progress)
	if err != nil {
		return "", err
	}

	return res.fileName, nil
}

// result — разобранный вывод скрипта.
type result struct {
	fileName string
	media    []Media
}

// run запускает bash-скрипт с аргументами, парсит [INFO], [ID], [MEDIA], [ERROR] и [CODE].
func (d *BashDownloader) run(ctx context.Context, args []string, progress func(string)) (result, error) {
	cmd := exec.CommandContext(ctx, "bash", append([]string{d.scriptPath}, args...)...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return result{}, err
	}

	if err := cmd.Start(); err != nil {
		return result{}, err
	}

	var (
		wg     sync.WaitGroup
		res    result
		reason string
		code   string
	)

	wg.Add(2)

	go func() {
		defer wg.Done()

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			s := scanner.Text()
			switch {
			case strings.HasPrefix(s, "[INFO]: "):
				if progress != nil {
					progress(strings.TrimPrefix(s, "[INFO]: "))
				}
			case strings.HasPrefix(s, "[ID]: "):
				res.fileName = strings.TrimPrefix(s, "[ID]: ")
			case strings.HasPrefix(s, "[MEDIA]: "):
				if m, ok := parseMedia(strings.TrimPrefix(s, "[MEDIA]: ")); ok {
					res.media = append(res.media, m)
				}
			default:
				slog.DebugContext(ctx, s)
			}
		}
	}()

	go func() {
		defer wg.Done()

		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			s := scanner.Text()
			switch {
			case strings.HasPrefix(s, "[ERROR]: "):
				reason = strings.TrimPrefix(s, "[ERROR]: ")
			case strings.HasPrefix(s, "[CODE]: "):
				code = strings.TrimPrefix(s, "[CODE]: ")
			}
			slog.ErrorContext(ctx, s)
		}
	}()

	wg.Wait()

	if wErr := cmd.Wait(); wErr != nil {
		if reason != "" {
			return result{}, &ScriptError{Code: code, Reason: reason, Err: wErr}
		}
		return result{}, wErr
	}

	return res, nil
}

// parseMedia разбирает строку "<номер>|<тип>".
func parseMedia(raw string) (Media, bool) {
	parts := strings.SplitN(raw, "|", 2)
	if len(parts) != 2 {
		return Media{}, false
	}

	index, err := strconv.Atoi(parts[0])
	if err != nil {
		return Media{}, false
	}

	return Media{Index: index, Kind: parts[1]}, true
}
