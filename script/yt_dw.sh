#!/usr/bin/env bash
set -Eeuo pipefail

err() { echo "[ERROR]: $*" >&2; }
info() { echo "[INFO]: $*"; }
debug() { echo "[DEBUG]: $*"; }
die() { err "$1"; exit "${2:-1}"; }

usage() {
  cat >&2 <<USAGE
Usage: $(basename "$0") <url>
Скачивает видео:
- Папка: ${SAVE_DIR}
- Разрешение: <=720p (предпочтительно h264, но не строго; есть fallback)
- Контейнер: mp4 при возможности (без перекодирования)
- Cookies: из файла cookies.txt
USAGE
}

URL="${1:-}"
[[ -z "${URL}" ]] && { usage; exit 1; }

SAVE_DIR="${SAVE_DIR:-/var/tmp/yt_dw}"

# Проверка зависимостей
command -v yt-dlp >/dev/null 2>&1 || die "Отсутствует yt-dlp" 4
HAS_FFMPEG=0
if command -v ffmpeg >/dev/null 2>&1; then
  HAS_FFMPEG=1
else
  debug "Предупреждение: ffmpeg не найден — объединение/ремукс недоступны, попробую готовые форматы."
fi

# Сетевые параметры
RETRIES="${RETRIES:-5}"
FRAG_RETRIES="${FRAG_RETRIES:-10}"
SOCKET_TIMEOUT="${SOCKET_TIMEOUT:-20}"
CONCURRENT_FRAG="${CONCURRENT_FRAG:-5}"

# Каталог сохранения
mkdir -p "${SAVE_DIR}" 2>/dev/null || die "Не удалось создать каталог: ${SAVE_DIR}"

# Хост (информативная проверка)
HOST="$(printf '%s' "${URL}" | sed -E 's#^[a-zA-Z]+://##' | cut -d/ -f1 | cut -d: -f1 || true)"

if [[ -n "${HOST}" ]] && command -v ping >/dev/null 2>&1; then
    info "Проверяю доступность ${HOST}..."

    if ! ping -c 1 -W 2 "${HOST}" >/dev/null 2>&1; then
        info "${HOST} недоступен 😔"
        sleep 1s
        die "${HOST} недоступен."
    fi
fi

# Опциональные cookies: по умолчанию выключены; если есть cookies.txt рядом — используем.
NO_COOKIES="0"   # 1 — игнорировать cookies даже если файл существует
COOKIES_FILE="/app/script/cookies.txt"
DENO_PATH="$HOME/.deno/bin/deno"

if [[ "$NO_COOKIES" != "1" && -s "$COOKIES_FILE" ]]; then
  # Рекомендация по безопасным правам
  perm="$(stat -c '%a' "$COOKIES_FILE" 2>/dev/null || stat -f '%Lp' "$COOKIES_FILE" 2>/dev/null || echo "")"
  if [[ -n "$perm" && "$perm" != "600" && "$perm" != "400" ]]; then
    debug "ВНИМАНИЕ: установите права 600 на $COOKIES_FILE (содержит приватные данные)"
  fi
  debug "Использую cookies: $COOKIES_FILE"
else
  debug "Cookies отключены (по умолчанию)."
fi

if [[ -n "$DENO_PATH" && -x "$DENO_PATH" ]]; then
        debug "Использую js runtime: $DENO_PATH"
    else
        debug "ВНИМАНИЕ: для скачивания видео из Youtube требуется установленный js runtime. Например, Deno"
fi

yt-dlp \
    --simulate --skip-download --quiet --no-warnings \
    --cookies "${COOKIES_FILE}" \
    --js-runtimes "deno:${DENO_PATH}" \
    --socket-timeout "${SOCKET_TIMEOUT}" \
    --retries "${RETRIES}" \
    -o '%(id)s.%(ext)s' \
    --print "[ID]: %(id)s.%(ext)s" \
    "${URL}"

# Формат: предпочитаем h264 ≤720p, далее любой ≤720p, затем любой best.
# Для наличия ffmpeg пробуем мердж в mp4, если совместимо.
if [[ "${HAS_FFMPEG}" -eq 1 ]]; then
  FORMAT="bv*[height<=720][vcodec~='^(avc1|h264)']+ba/b[height<=720][vcodec~='^(avc1|h264)']/bv*[height<=720]+ba/b[height<=720]/b"
  MERGE_ARGS=(--merge-output-format mp4)
else
  # Без ffmpeg берём готовый единый поток (предпочт. mp4/h264), затем любой ≤720p, затем best
  FORMAT="b[height<=720][ext=mp4][vcodec~='^(avc1|h264)']/b[height<=720]/b"
  MERGE_ARGS=()
fi

MAX_SIZE_MB=50M

info "Скачиваю..."

yt-dlp \
  -f "${FORMAT}" \
  "${MERGE_ARGS[@]}" \
  --cookies "${COOKIES_FILE}" \
  --js-runtimes "deno:${DENO_PATH}" \
  --no-playlist \
  --quiet \
  --no-warnings \
  --max-filesize "${MAX_SIZE_MB}" \
  --retries "${RETRIES}" \
  --fragment-retries "${FRAG_RETRIES}" \
  --socket-timeout "${SOCKET_TIMEOUT}" \
  --concurrent-fragments "${CONCURRENT_FRAG}" \
  -o "${SAVE_DIR}/%(id)s.%(ext)s" \
  "${URL}"