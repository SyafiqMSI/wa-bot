FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy go mod files first for better caching
COPY go.mod ./
COPY go.sum ./

RUN go mod download

# Copy the rest of the application
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o wa-bot .

FROM alpine:3.20

WORKDIR /root/

RUN apk add --no-cache sqlite-libs libwebp-tools

COPY --from=builder /app/wa-bot .

RUN chmod +x ./wa-bot

CMD ["./wa-bot"]