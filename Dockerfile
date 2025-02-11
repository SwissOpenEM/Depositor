# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.22 AS builder

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY main.go ./
COPY conversions.csv /app/conversions.csv
COPY mmcif_pdbx_v50.dic /app/mmcif_pdbx_v50.dic
COPY depositions/onedep ./depositions/onedep
COPY docs ./docs

ARG VERSION=v.0.1.0

# Build GOARCH=amd64
RUN CGO_ENABLED=0 GOOS=linux  GOARCH=${TARGETARCH} go generate .
RUN CGO_ENABLED=0 GOOS=linux  GOARCH=${TARGETARCH} go build -o /app/depositor

# Final stage
FROM alpine:latest

# Set environment variables
ENV PORT=8888
ENV ALLOW_ORIGINS=http://localhost:4200

# Copy the binary and other necessary files from the builder stage
COPY --from=builder /app/depositor /app/depositor
COPY --from=builder /app/conversions.csv /app/conversions.csv
COPY --from=builder /app/mmcif_pdbx_v50.dic /app/mmcif_pdbx_v50.dic


EXPOSE 8888
# Run
WORKDIR /app
CMD ["./depositor"]