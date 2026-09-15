# yt_dw — инструкции для агентов

Telegram-бот (один бинарник, `main.go` в корне): принимает ссылку в чате,
скачивает видео через `bash script/yt_dw.sh` (yt-dlp) и отправляет обратно.
Лимиты: ≤720p, ≤50 МБ, mp4 без перекодирования.

## Команды

- `go test ./...` — основной прогон; ровно это делает CI. Тесты офлайн: сеть, Telegram и yt-dlp не нужны.
- Перед завершением: `gofmt -l .`, `go vet ./...`, `go build ./...`, `go test -race ./...` (всё проходит на go1.26).
- `gofmt -l .` уже выдаёт `config/config.go`, `internal/stats/stats_test.go`, `internal/telegram/mock_test.go`, `internal/validator/validator_test.go` — это давний долг. Не форматировать их попутно, только файлы своих изменений.
- Один тест: `go test ./internal/telegram -run TestHandler_Link -v`.
- Запуск `go run .` — только из корня репозитория: `application.yaml`, `script/yt_dw.sh` и `/var/tmp/yt_dw` захардкожены относительно CWD. Без `application.yaml` будет `log.Fatalf` (даже если все секреты в env).

## Устройство

- `main.go` собирает цепочку: `config` → `logger` → `downloader.New("script/yt_dw.sh")` → `filestore.New("/var/tmp/yt_dw/")` → `stats.New("/var/tmp/yt_dw/stats.json")` → `telegram.NewHandler(...)` → long-polling `go-telegram/bot` (`/start`, `/stats`, префикс `https://`, callback `ig:`, default).
- Пакеты по домену: `internal/downloader` (запуск скрипта), `internal/filestore`, `internal/stats` (JSON, автосейв раз в 30 с и по ctx; username и домены по юзерам для `/stats`), `internal/telegram` (handler/client/messages), `internal/validator`, `internal/logger`.
- `telegram.BotClient` — интерфейс для ручных моков (`mock_test.go`); `downloader.Downloader` и `filestore.FileStore` — интерфейсы, которые потребляет `telegram`.

## Контракт Go ↔ yt_dw.sh

- `downloader` читает stdout построчно: `[INFO]: ...` → callback прогресса (правит сообщение в Telegram), `[ID]: <файл>` → имя файла (побеждает последняя строка), `[MEDIA]: <N>|<photo|video>` → элементы поста (`List`), прочее → debug. Со stderr: `[ERROR]: <текст>` → `downloader.ScriptError.Reason` (идёт в чат), `[CODE]: <код>` → `.Code` (идёт в `ErrorStats`), остальные строки → error-лог. Каждая ошибка скрипта помечена кодом (`size_limit`, `video_id_error`, `yt_dlp_error`, `media_list_error`, …). Меняя вывод скрипта, обнови парсер и `downloader_test.go`.
- Режимы скрипта: без флагов — видео; `--list <url>` — элементы поста; `--item N <url>` — конкретный элемент. Для `instagram.com/.../p|tv/<shortcode>/` Handler зовёт `List`: один элемент — сразу, несколько — инлайн-кнопки с callback data `ig:<shortcode>:<N>`, выбор обрабатывает `Handler.PickMedia`. Reels и остальные ссылки идут обычным путём.
- `Handler.sendMedia` по расширению выбирает транспорт: `.jpg/.jpeg/.png/.webp` → `SendPhoto` (имя как есть), иначе принудительно `.mp4` + `SendVideo`. `sendVideo` больше нет.
- Скрипту нужен `bash`; `ffmpeg` опционален (fallback на готовый поток), `curl` обязателен для скачивания фото (`--item` по картинке), deno ищется в `$HOME/.deno/bin/deno`, cookies — жёстко `/app/script/cookies.txt`.
- Env скрипта: `SAVE_DIR` (по умолчанию `/var/tmp/yt_dw`), `RETRIES`, `FRAG_RETRIES`, `SOCKET_TIMEOUT`, `CONCURRENT_FRAG`. `MAX_SIZE_MB` env не читает: в скрипте жёстко `50`, вопреки README.
- До загрузки скрипт оценивает размер выбранного формата (`filesize` → `filesize_approx` → `tbr × duration`) и падает с `[CODE]: size_limit`, не качая файл. Для direct-форматов yt-dlp сам отсекает по `Content-Length`, для HLS оценка по `tbr` — единственная защита, `--max-filesize` на HLS не работает.

## Конфигурация

- `application.yaml`: `telegram.token`, `telegram.admin_ids`, `log.level`, `log.format`. Env `TELEGRAM_TOKEN` и `TELEGRAM_ADMIN_IDS` (через запятую) переопределяют YAML.
- `application.dev.yaml` и `script/cookies.txt` в gitignore — секреты не коммитить.

## Docker и CI

- `docker build --build-arg PROFILE=dev` запекает `application.dev.yaml` и cookies; prod (по умолчанию) берёт секреты из env, cookies монтируются в `/app/script/cookies.txt`. Контейнер работает от root, поэтому yt-dlp делает host-файл cookies root-owned.
- Образ каждый раз качает `latest` yt-dlp/deno/ffmpeg при сборке — воспроизводимости нет.
- `.github/workflows/ci-cd.yaml`: push/PR в `main` (изменения только `*.md`/`docs/**` игнорируются) → `go test ./...` → push в ghcr `latest` и `sha-<sha>` → SSH-деплой (stop/rm/run). Линтера, vet и `-race` в CI нет — гонять локально.

## Соглашения

- Комментарии/godoc и тексты бота — на русском, идентификаторы — на английском, коммиты — английский императив.
- Тесты: без assert-библиотек, моки руками, table-driven с `t.Run`, временные файлы через `t.TempDir`; ничего, что требует сеть или внешние бинарники.
