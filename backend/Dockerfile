FROM golang:1.23-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/haohaoaccounting \
    ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S app -G app

WORKDIR /app

COPY --from=builder /out/haohaoaccounting /app/haohaoaccounting

ENV GIN_MODE=release
ENV PORT=8080

EXPOSE 8080

USER app

ENTRYPOINT ["/app/haohaoaccounting"]
