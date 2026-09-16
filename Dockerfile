# syntax=docker/dockerfile:1.7

FROM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS app-builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY config ./config
COPY frontend ./frontend
COPY internal ./internal

RUN export CGO_ENABLED=0 \
      GOOS="${TARGETOS:-$(go env GOOS)}" \
      GOARCH="${TARGETARCH:-$(go env GOARCH)}" && \
    go build -trimpath -ldflags="-s -w" -o /out/conduit ./cmd/server
RUN export CGO_ENABLED=0 \
      GOOS="${TARGETOS:-$(go env GOOS)}" \
      GOARCH="${TARGETARCH:-$(go env GOARCH)}" && \
    go build -trimpath -ldflags="-s -w" -o /out/frontend ./cmd/frontend

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

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S -g 10001 conduit && \
    adduser -S -D -H -u 10001 -G conduit conduit

WORKDIR /app

COPY --from=app-builder /out/conduit /app/conduit
COPY --from=app-builder /out/frontend /app/frontend
COPY --from=goose-builder /out/goose /app/goose
COPY --chown=conduit:conduit config/config.yaml /app/config/config.yaml
COPY --chown=conduit:conduit config/frontend.yaml /app/config/frontend.yaml
COPY --chown=conduit:conduit migrations /app/migrations

USER conduit:conduit

EXPOSE 3000 8000

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget --quiet --spider http://127.0.0.1:8000/metrics || exit 1

ENTRYPOINT ["/app/conduit"]
CMD ["--addr=:8000"]
