# ============================================================
# Stage 1: Build
# ============================================================
FROM golang:1.26 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
        -ldflags="-s -w" \
        -o /out/app .

# ============================================================
# Stage 2: Runtime
# ============================================================
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        ffmpeg \
        python3 \
        pipx \
        unzip && \
    rm -rf /var/lib/apt/lists/*

# -------------------------
# yt-dlp
# -------------------------
RUN curl -L \
    https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp \
    -o /usr/local/bin/yt-dlp && \
    chmod +x /usr/local/bin/yt-dlp

# -------------------------
# Deno
# -------------------------
RUN curl -fsSL https://deno.land/install.sh | sh

# -------------------------
# Temporary storage
# -------------------------
RUN mkdir -p /var/tmp/yt_dw

VOLUME ["/var/tmp/yt_dw"]

# -------------------------
# Application
# -------------------------
RUN mkdir -p /app
WORKDIR /app

COPY --from=builder /out/app ./app
COPY --from=builder /src/application.yaml ./application.yaml
COPY --from=builder /src/script/ ./script/

ENTRYPOINT ["/app/app"]