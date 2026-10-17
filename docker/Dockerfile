# syntax=docker/dockerfile:1
#
# Builds the Cliparr server with the web frontend bundled in.
# Build from the repository root:  docker build -f docker/Dockerfile .
# Normally you want `just docker-build`, which also passes the right build args.

ARG FLUTTER_VERSION=3.47.7

# --- 1. Flutter web frontend (runs on the build machine, even for arm64 images) ---
FROM --platform=$BUILDPLATFORM debian:bookworm-slim AS web
ARG FLUTTER_VERSION
ARG VERSION=dev
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git unzip xz-utils \
    && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL "https://storage.googleapis.com/flutter_infra_release/releases/stable/linux/flutter_linux_${FLUTTER_VERSION}-stable.tar.xz" \
    | tar -xJ -C /opt
ENV PATH="/opt/flutter/bin:${PATH}"
RUN git config --global --add safe.directory /opt/flutter \
    && flutter config --no-analytics --enable-web \
    && flutter precache --web
WORKDIR /src/app
COPY app/pubspec.yaml app/pubspec.lock ./
RUN flutter pub get
COPY app/ ./
RUN flutter build web --release --dart-define=CLIPARR_VERSION=${VERSION}

# --- 2. Go server (cross-compiled, no CGO) ---
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS server
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY --from=web /src/app/build/web/ internal/web/dist/
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/cliparr ./cmd/cliparr

# --- 3. Runtime image: the binary plus ffmpeg for the clip preview pictures ---
FROM alpine:3.22
LABEL org.opencontainers.image.source="https://github.com/bergercookie/cliparr" \
      org.opencontainers.image.description="Save and replay your favorite Jellyfin moments"
RUN apk add --no-cache ffmpeg ca-certificates \
    && adduser -D -H -u 65532 cliparr \
    && mkdir /data \
    && chown cliparr /data
COPY --from=server /out/cliparr /usr/local/bin/cliparr
ENV CLIPARR_DATA_DIR=/data \
    CLIPARR_LISTEN=:8080
VOLUME /data
EXPOSE 8080
USER cliparr
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD ["cliparr", "healthcheck"]
ENTRYPOINT ["cliparr"]
