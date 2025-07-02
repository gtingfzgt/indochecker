# Dockerfile (Final, Robust Version)

# Stage 1: Build the Go application
FROM golang:1.21-alpine AS builder
WORKDIR /app

# Copy the module definition file first
COPY go.mod ./

# Copy the application code
COPY main.go ./

# Automatically manage dependencies and create a go.sum file
RUN go mod tidy

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bot .

# Stage 2: Create the final small image
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/bot /app/bot

# Add SSL certificates so our app can make HTTPS requests
RUN apk --no-cache add ca-certificates

# Run our bot
CMD ["/app/bot"]
