# syntax=docker/dockerfile:1.7

FROM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS app-builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY config ./config
COPY internal ./internal

RUN export CGO_ENABLED=0 \
      GOOS="${TARGETOS:-$(go env GOOS)}" \
      GOARCH="${TARGETARCH:-$(go env GOARCH)}" && \
    go build -trimpath -ldflags="-s -w" -o /out/conduit ./cmd/server

FROM node:24-alpine@sha256:ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1 AS frontend-builder

WORKDIR /src/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend ./
COPY api /src/api
RUN npm run build

FROM nginx:1.29-alpine@sha256:5616878291a2eed594aee8db4dade5878cf7edcb475e59193904b198d9b830de AS frontend-runtime

COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=frontend-builder /src/frontend/dist /usr/share/nginx/html

EXPOSE 3000

FROM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS goose-builder

ARG TARGETOS
ARG TARGETARCH
ARG GOOSE_VERSION=v3.28.0

WORKDIR /src

RUN go mod init conduit-goose-build && \
    go mod edit -require="github.com/pressly/goose/v3@${GOOSE_VERSION}" && \
    go mod download github.com/pressly/goose/v3
RUN export CGO_ENABLED=0 \
      GOOS="${TARGETOS:-$(go env GOOS)}" \
      GOARCH="${TARGETARCH:-$(go env GOARCH)}" && \
    go build -mod=mod \
      -tags="no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb" \
      -trimpath -ldflags="-s -w" \
      -o /out/goose github.com/pressly/goose/v3/cmd/goose

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40 AS api-runtime

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S -g 10001 conduit && \
    adduser -S -D -H -u 10001 -G conduit conduit

WORKDIR /app

COPY --from=app-builder /out/conduit /app/conduit
COPY --from=goose-builder /out/goose /app/goose
COPY --chown=conduit:conduit config/config.yaml /app/config/config.yaml
COPY --chown=conduit:conduit migrations /app/migrations

USER conduit:conduit

EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget --quiet --spider http://127.0.0.1:8000/metrics || exit 1

ENTRYPOINT ["/app/conduit"]
CMD ["--addr=:8000"]
