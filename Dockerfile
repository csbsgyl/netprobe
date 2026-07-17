# syntax=docker/dockerfile:1
FROM node:22-alpine AS web-build
WORKDIR /src
RUN corepack enable && corepack prepare pnpm@10.13.1 --activate
COPY pnpm-workspace.yaml pnpm-lock.yaml ./
COPY web/package.json web/package.json
RUN pnpm install --frozen-lockfile
COPY web ./web
RUN pnpm --dir web build

FROM golang:1.22-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/netprobe-server ./cmd/netprobe-server
RUN mkdir -p /out/downloads
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/downloads/netcheck-linux-amd64 ./cmd/netcheck
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/downloads/netcheck-linux-arm64 ./cmd/netcheck
RUN CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/downloads/netcheck-windows-amd64.exe ./cmd/netcheck
RUN CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/downloads/netcheck-windows-arm64.exe ./cmd/netcheck
RUN cd /out/downloads && sha256sum netcheck-* > /tmp/checksums && \
    while read -r sum file; do printf '%s  %s\n' "$sum" "$file" > "$file.sha256"; done < /tmp/checksums

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && addgroup -S netprobe && adduser -S -G netprobe netprobe
COPY --from=build /out/netprobe-server /usr/local/bin/netprobe-server
COPY --from=build /out/downloads /srv/downloads
COPY --from=web-build /src/web/dist /srv/web
USER netprobe
EXPOSE 8080/tcp 3478/udp 3479/udp
ENTRYPOINT ["/usr/local/bin/netprobe-server"]
