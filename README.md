```
       _          _          
 _   _| |_     __| |_      __
| | | | __|   / _` \ \ /\ / /
| |_| | |_   | (_| |\ V  V / 
 \__, |\__|___\__,_| \_/\_/  
 |___/   |_____| 
```

A Telegram bot that downloads videos by link and sends them straight to the chat.
Limits: ≤720p, ≤50MB, mp4 without re-encoding; powered by yt-dlp
(YouTube and hundreds of other sites).

## Commands

- `/start` — greeting
- `/stats` — statistics (admins only, see `admin_ids`)
- send a link — download and delivery of the video

## Quick start (Docker, ghcr.io)

```bash
docker run -d \
  --name yt_dw \
  --restart unless-stopped \
  -v /var/tmp/yt_dw:/var/tmp/yt_dw \
  -e TELEGRAM_TOKEN="<token>" \
  ghcr.io/drsdgdbye/yt_dw:latest
```

Image tags: `latest`, `sha-<commit>`.

## Configuration

Required (one of two):

- `application.yaml` — `token`, `admin_ids`, `log` (see `application.yaml`)
- env vars: `TELEGRAM_TOKEN`, `TELEGRAM_ADMIN_IDS` (override YAML)

Optional:

- `script/cookies.txt` — 18+ / authenticated content (permissions 600).
  In the container it is mounted read-only from the server:
  place the file on the VDS at `/var/tmp/yt_dw/cookies.txt`
  (e.g. `scp script/cookies.txt user@host:/var/tmp/yt_dw/cookies.txt`).
- script env vars: `SAVE_DIR`, `RETRIES`, `FRAG_RETRIES`, `MAX_SIZE_MB`, etc.
- `log.level` / `log.format` in `application.yaml`

## Storage

Volume `/var/tmp/yt_dw` — downloaded files and `stats.json`.

`stats.json` — bot statistics: new chats, processed links, successes/errors
(per chat), top domains, error types, average file size and processing time.
Auto-saved every 30s and on shutdown; used by the `/stats` command.
