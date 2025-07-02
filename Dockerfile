# Dockerfile

# --- Stage 1: Build the 'checkdomain' tool ---
FROM golang:1.21-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src/checkdomain
RUN git clone https://github.com/Skiddle-ID/checkdomain.git .
RUN go build -o /checkdomain .

# --- Stage 2: Build our bot application ---
FROM golang:1.21-alpine
WORKDIR /app
COPY --from=builder /checkdomain /app/checkdomain
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .
RUN go build -o /app/bot .

# --- Final Stage: Run the application ---
FROM alpine:latest
WORKDIR /app
COPY --from=1 /app/checkdomain /app/checkdomain
COPY --from=1 /app/bot /app/bot
CMD ["/app/bot"]
