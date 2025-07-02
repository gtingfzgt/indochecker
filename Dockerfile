# Dockerfile (Simple version for Go app - with tidy fix)

# Stage 1: Build the Go application
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./

# --- THIS IS THE FIX ---
RUN go mod tidy
# --- END FIX ---

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bot .

# Stage 2: Create the final small image
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/bot /app/bot

# Add SSL certificates so our Go app can make HTTPS requests
RUN apk --no-cache add ca-certificates

# Run our bot
CMD ["/app/bot"]
