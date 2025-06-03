# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.23 AS builder

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
COPY /data/6z6u.pdb /app/data/6z6u.pdb
COPY /app/data/empiar_deposition.schema.json /app/data/empiar_deposition.schema.json

ARG VERSION=v.0.1.0

# Build GOARCH=amd64
RUN CGO_ENABLED=0 GOOS=linux  GOARCH=${TARGETARCH} go generate .
RUN CGO_ENABLED=0 GOOS=linux  GOARCH=${TARGETARCH} go build -o /app/depositor

# Stage 2: download pdb_extract 
FROM alpine:3.20 AS extractor

WORKDIR /opt/pdb_extract

RUN apk add --no-cache curl tar
RUN VERSION=$(curl -s https://sw-tools.rcsb.org/apps/PDB_EXTRACT/pdb-extract-latest-version.txt) && \
    curl -LO https://sw-tools.rcsb.org/apps/PDB_EXTRACT/pdb_extract_prod_py-${VERSION}.tar.gz && \
    tar -xzf pdb_extract_prod_py-${VERSION}.tar.gz

# Final stage using slim because maxit is incompatible with alpine
FROM python:3.12-slim AS final

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    bash \
    curl \
    build-essential \
    cmake \
    flex \
    bison \
    && rm -rf /var/lib/apt/lists/*

# Copy Go app
COPY --from=builder /app/depositor /app/depositor
COPY --from=builder /app/data/conversions.csv /app/data/conversions.csv
COPY --from=builder /app/data/mmcif_pdbx_v50.dic /app/data/mmcif_pdbx_v50.dic
COPY --from=builder /app/data/6z6u.pdb /app/data/6z6u.pdb
COPY --from=builder /app/README.md /app/README.md
#WILL REMOVE LATER
COPY --from=builder /app/data/empiar_deposition.schema.json /app/data/empiar_deposition.schema.json
# Copy extracted pdb_extract directory
COPY --from=extractor /opt/pdb_extract/pdb_extract_prod_py-* /app/scripts/


# Set up pdb_extract environment
WORKDIR /app/scripts
RUN pip install --upgrade pip && \
    pip install -r REQUIREMENTS.txt && \
    bash install.sh

WORKDIR /app

ENV PORT=8080
ENV ALLOW_ORIGINS=http://localhost:4201
ENV PYTHON=/app/scripts/extractor/bin/python
ENV RCSBROOT=/app/scripts/packages/maxit-v11.300-prod-src

EXPOSE 8080

CMD ["./depositor"]
