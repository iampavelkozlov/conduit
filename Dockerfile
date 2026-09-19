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
RUN export CGO_ENABLED=0 \
      GOOS="${TARGETOS:-$(go env GOOS)}" \
      GOARCH="${TARGETARCH:-$(go env GOARCH)}" && \
    go build -trimpath -ldflags="-s -w" -o /out/auth ./cmd/auth && \
    go build -trimpath -ldflags="-s -w" -o /out/profile ./cmd/profile && \
    go build -trimpath -ldflags="-s -w" -o /out/posts ./cmd/posts && \
    go build -trimpath -ldflags="-s -w" -o /out/subscriptions ./cmd/subscriptions && \
    go build -trimpath -ldflags="-s -w" -o /out/comments ./cmd/comments && \
    go build -trimpath -ldflags="-s -w" -o /out/outbox-relay ./cmd/outbox-relay

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
COPY --from=app-builder /out/auth /app/auth
COPY --from=app-builder /out/profile /app/profile
COPY --from=app-builder /out/posts /app/posts
COPY --from=app-builder /out/subscriptions /app/subscriptions
COPY --from=app-builder /out/comments /app/comments
COPY --from=app-builder /out/outbox-relay /app/outbox-relay
COPY --from=goose-builder /out/goose /app/goose
COPY --chown=conduit:conduit config/config.yaml /app/config/config.yaml
COPY --chown=conduit:conduit config/frontend.yaml /app/config/frontend.yaml
COPY --chown=conduit:conduit config/auth.yaml /app/config/auth.yaml
COPY --chown=conduit:conduit config/profile.yaml /app/config/profile.yaml
COPY --chown=conduit:conduit config/posts.yaml /app/config/posts.yaml
COPY --chown=conduit:conduit config/subscriptions.yaml /app/config/subscriptions.yaml
COPY --chown=conduit:conduit config/comments.yaml /app/config/comments.yaml
COPY --chown=conduit:conduit config/outbox-relay.yaml /app/config/outbox-relay.yaml
COPY --chown=conduit:conduit migrations /app/migrations
COPY --chown=conduit:conduit services/auth/migrations /app/migrations/auth
COPY --chown=conduit:conduit services/profile/migrations /app/migrations/profile
COPY --chown=conduit:conduit services/posts/migrations /app/migrations/posts
COPY --chown=conduit:conduit services/subscriptions/migrations /app/migrations/subscriptions
COPY --chown=conduit:conduit services/comments/migrations /app/migrations/comments

USER 10001:10001

EXPOSE 3000 8000 9001 9002 9003 9004 9005 9100

ENTRYPOINT ["/app/conduit"]
CMD ["--addr=:8000"]
