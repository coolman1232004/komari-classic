FROM golang:1.26.8-alpine AS cloudflared-build
RUN apk add --no-cache curl ca-certificates
WORKDIR /src/cloudflared
# Official 2026.9.1 source, pinned by commit and archive checksum.
RUN curl -fsSL https://codeload.github.com/cloudflare/cloudflared/tar.gz/f11dea9cb7079e90a982c1a2d5548ab40847fdcf -o /tmp/cloudflared.tar.gz \
    && echo "d9c67e530861b212529fe67f4d7fe04335dc80f07026370a57c46fa611f7730f  /tmp/cloudflared.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/cloudflared.tar.gz --strip-components=1
COPY third_party/cloudflared/go.mod third_party/cloudflared/go.sum ./
RUN CGO_ENABLED=0 go test -mod=readonly ./tracing
RUN CGO_ENABLED=0 go build -mod=readonly -trimpath -ldflags="-s -w -X main.Version=2026.9.1-classic-security -X github.com/cloudflare/cloudflared/cmd/cloudflared/updater.BuiltForPackageManager=komari-classic" -o /out/cloudflared ./cmd/cloudflared

FROM alpine:3.24

WORKDIR /app

# Docker buildx 会在构建时自动填充这些变量
ARG TARGETOS
ARG TARGETARCH

# Upgrade packages already present in the base; reject mirrors missing the security fix.
RUN apk upgrade --no-cache \
    && apk add --no-cache 'libcrypto3>=3.5.8-r0' 'libssl3>=3.5.8-r0' ca-certificates curl tzdata

COPY --from=cloudflared-build /out/cloudflared /usr/local/bin/cloudflared
COPY --from=cloudflared-build /src/cloudflared/LICENSE /usr/share/licenses/cloudflared/LICENSE
RUN cloudflared --version

COPY komari-${TARGETOS}-${TARGETARCH} /app/komari

RUN chmod +x /app/komari

ENV GIN_MODE=release
ENV KOMARI_DB_TYPE=sqlite
ENV KOMARI_DB_FILE=/app/data/komari.db
ENV KOMARI_DB_HOST=localhost
ENV KOMARI_DB_PORT=3306
ENV KOMARI_DB_USER=root
ENV KOMARI_DB_PASS=
ENV KOMARI_DB_NAME=komari
ENV KOMARI_LISTEN=0.0.0.0:25774

EXPOSE 25774

CMD ["/app/komari", "server"]
