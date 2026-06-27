FROM golang:1.23-bookworm AS builder

WORKDIR /src

ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -buildvcs=false \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/haohaoaccounting \
    ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build \
    -buildvcs=false \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/dbmigrate \
    ./cmd/dbmigrate

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S app -G app

WORKDIR /app

COPY --from=builder /out/haohaoaccounting /app/haohaoaccounting
COPY --from=builder /out/dbmigrate /app/dbmigrate
COPY --from=builder /src/migrations /app/migrations

ENV GIN_MODE=release
ENV PORT=8080

EXPOSE 8080

USER app

ENTRYPOINT ["/app/haohaoaccounting"]
