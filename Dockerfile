FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /soulgate ./cmd/soulgate/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates chromium ffmpeg
COPY --from=builder /soulgate /usr/local/bin/soulgate
RUN mkdir -p /root/.soulgate
EXPOSE 8080
ENTRYPOINT ["soulgate"]
CMD ["gateway", "start"]
