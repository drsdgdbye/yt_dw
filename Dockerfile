# ============================================================
# Stage 1: Build
# ============================================================
FROM golang:1.26 AS builder

ARG PROFILE=prod

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
        -ldflags="-s -w" \
        -o /out/app .

# Подготовка рантайм-файлов под профиль
RUN mkdir -p /out/rt/script && \
    cp script/yt_dw.sh /out/rt/script/yt_dw.sh && \
    if [ "${PROFILE}" = "dev" ]; then \
        cp application.dev.yaml /out/rt/application.yaml && \
        cp script/cookies.txt /out/rt/script/cookies.txt; \
    else \
        cp application.yaml /out/rt/application.yaml; \
    fi

# ============================================================
# Stage 2: Downloads (binaries only, not shipped)
# ============================================================
FROM debian:12-slim AS downloads

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        unzip \
        xz-utils && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /dl/.deno/bin

# yt-dlp (standalone binary, no python needed)
RUN curl -L \
    https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux \
    -o /dl/yt-dlp && \
    chmod +x /dl/yt-dlp

# Deno — ставим в /root/.deno/bin/deno (путь, который ждёт yt_dw.sh)
RUN curl -fsSL https://deno.land/install.sh | sh && \
    cp /root/.deno/bin/deno /dl/.deno/bin/deno

# ffmpeg (static build, glibc)
RUN curl -L \
    https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz \
    -o /tmp/ffmpeg.tar.xz && \
    tar -xJf /tmp/ffmpeg.tar.xz \
        -C /dl \
        --strip-components=1 \
        --wildcards '*/ffmpeg'

# ============================================================
# Stage 3: Runtime
# ============================================================
FROM debian:12-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates && \
    rm -rf /var/lib/apt/lists/*

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

COPY --from=downloads /dl/yt-dlp /usr/local/bin/yt-dlp
COPY --from=downloads /dl/.deno/bin/deno /root/.deno/bin/deno
COPY --from=downloads /dl/ffmpeg /usr/local/bin/ffmpeg

COPY --from=builder /out/app ./app
COPY --from=builder /out/rt/application.yaml ./application.yaml
COPY --from=builder /out/rt/script/ ./script/

ENTRYPOINT ["/app/app"]
