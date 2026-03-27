# syntax=docker/dockerfile:1

FROM oven/bun:1.2-alpine AS ui
WORKDIR /ui
COPY apps/user/package.json apps/user/bun.lock ./
RUN --mount=type=cache,target=/root/.bun/install/cache bun install --frozen-lockfile
COPY apps/user/ ./
RUN bun run build

FROM oven/bun:1.2-alpine AS admin
WORKDIR /admin
COPY apps/admin/package.json apps/admin/bun.lock ./
RUN --mount=type=cache,target=/root/.bun/install/cache bun install --frozen-lockfile
COPY apps/admin/ ./
RUN bun run build

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=ui /ui/dist ./apps/user/dist
COPY --from=admin /admin/dist ./apps/admin/dist
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /writekit ./cmd/writekit

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /writekit /writekit
EXPOSE 8080
ENTRYPOINT ["/writekit"]
