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
COPY /data/conversions.csv /app/data/conversions.csv
COPY /data/mmcif_pdbx_v50.dic /app/data/mmcif_pdbx_v50.dic
COPY depositions/onedep ./depositions/onedep
COPY docs ./docs
COPY README.md /app/README.md

ARG VERSION=v.0.1.0

# Build GOARCH=amd64
RUN CGO_ENABLED=0 GOOS=linux  GOARCH=${TARGETARCH} go generate .
RUN CGO_ENABLED=0 GOOS=linux  GOARCH=${TARGETARCH} go build -o /app/depositor

# Stage 2: Python + pdb_extract builder
FROM python:3.12-alpine AS pybuilder

# Install python3 for OneDep-Empiar scripts, curl, bash etc
RUN apk add --no-cache \
    bash \
    curl \
    tar \
    coreutils \
    cmake \
    flex \
    bison \
    build-base \
    python3-dev

WORKDIR /opt/pdb_extract

# Download the latest version number of PDB-extractor-tool
RUN VERSION=$(curl -s https://sw-tools.rcsb.org/apps/PDB_EXTRACT/pdb-extract-latest-version.txt) && \
    curl -LO https://sw-tools.rcsb.org/apps/PDB_EXTRACT/pdb_extract_prod_py-${VERSION}.tar.gz && \
    tar -xzf pdb_extract_prod_py-${VERSION}.tar.gz && \
    cd pdb_extract_prod_py-${VERSION} && \
    python3 -m venv venv && \
    . venv/bin/activate && \
    pip install --upgrade pip && \
    pip install -r REQUIREMENTS.txt && \
    bash install.sh


# Final stage
FROM alpine:latest



# Set environment variables
ENV PORT=8888
ENV ALLOW_ORIGINS=http://localhost:4201

# Copy the binary and other necessary files from the builder stage
COPY --from=builder /app/depositor /app/depositor
COPY --from=builder /app/data/conversions.csv /app/data/conversions.csv
COPY --from=builder /app/data/mmcif_pdbx_v50.dic /app/data/mmcif_pdbx_v50.dic
COPY --from=builder /app/README.md /app/README.md
COPY --from=builder /app/depositor /app/depositor
COPY --from=builder /app/depositor /app/depositor

COPY --from=pybuilder /opt/pdb_extract/pdb_extract_prod_py-* /app/scripts/

WORKDIR /app

EXPOSE 8080
# Run
CMD ["./depositor"]