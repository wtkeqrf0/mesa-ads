FROM golang:1.24.4-alpine3.22 AS builder
RUN apk add --no-cache git make
WORKDIR /app
COPY go.mod go.sum ./
COPY Makefile Makefile
COPY cmd cmd
COPY internal internal
COPY migrations migrations
RUN make build

FROM ubuntu:noble-20250529
RUN useradd -r -m -s /sbin/nologin appuser
COPY --from=builder /app/mesa-ads /usr/local/bin/mesa-ads
RUN chown appuser:appuser /usr/local/bin/mesa-ads
USER appuser
ENTRYPOINT ["mesa-ads"]
