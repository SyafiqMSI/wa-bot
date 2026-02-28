FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev sqlite-dev

COPY go.mod ./
COPY go.sum ./

RUN go mod download
COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o wa-bot .

FROM alpine:3.20

WORKDIR /root/

RUN apk add --no-cache sqlite-libs libwebp-tools ffmpeg

COPY --from=builder /app/wa-bot .

RUN chmod +x ./wa-bot

CMD ["./wa-bot"]