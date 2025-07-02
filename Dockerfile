# Dockerfile (Corrected based on the final error)

# --- Stage 1: Build the 'checkdomain' tool ---
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src/checkdomain
# Clone the repository
RUN git clone https://github.com/Skiddle-ID/checkdomain.git .

# --- THIS IS THE CORRECT COMBINATION ---
# 1. Create the module context that the Go compiler needs.
RUN go mod init checkdomain && go mod tidy
# 2. Build from the current directory ('.') where main.go actually is.
RUN go build -o /checkdomain .


# --- Stage 2: Build our bot application (This part has always been correct) ---
FROM golang:1.21-alpine

WORKDIR /app

# Copy the pre-built 'checkdomain' executable from the builder stage
COPY --from=builder /checkdomain /app/checkdomain

# Copy Go module files for our bot
COPY go.mod go.sum ./
RUN go mod download

# Copy our bot's source code
COPY main.go .

# Build our bot application
RUN go build -o /app/bot .

# --- Final Stage: Run the application ---
FROM alpine:latest

WORKDIR /app

# Copy the two executables we need from the build stage
COPY --from=1 /app/checkdomain /app/checkdomain
COPY --from=1 /app/bot /app/bot

# The command to run when the container starts
CMD ["/app/bot"]
