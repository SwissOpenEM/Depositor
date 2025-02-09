# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.22 AS builder

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY ./depositions/onedep ./depositions/onedep
COPY ./conversions.csv /app/conversions.csv

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go generate ./depositions/onedep
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/depositor ./depositions/onedep

# Final stage
FROM alpine:latest

# Install necessary packages
RUN apk add --no-cache ca-certificates

# Set environment variables
ENV PORT=8080
ENV ALLOW_ORIGINS=http://localhost:4200

# Copy the binary from the builder stage
COPY --from=builder /app/depositor /app/depositor
COPY --from=builder /app/conversions.csv /app/conversions.csv

# Set executable permission for the depositor binary
RUN chmod +x /app/depositor

EXPOSE 8080
# Run
WORKDIR /app
CMD ["./depositor"]