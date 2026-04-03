# Stage 1: Build Go archlint binary
FROM golang:1.25-bookworm AS go-builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/archlint ./cmd/archlint/

# Stage 2: Build Rust archlint-rs binary
FROM rust:1.88-bookworm AS rs-builder

WORKDIR /build

COPY archlint-rs/Cargo.toml archlint-rs/Cargo.lock ./
# Cache dependencies
RUN mkdir src && echo "fn main() {}" > src/main.rs && cargo build --release && rm -rf src

COPY archlint-rs/src/ src/
# Touch main.rs to force rebuild
RUN touch src/main.rs && cargo build --release

RUN mkdir -p /out && cp target/release/archlint /out/archlint-rs

# Stage 3: Runtime image
FROM python:3.12-slim

LABEL org.opencontainers.image.source="https://github.com/mshogin/archlint"
LABEL org.opencontainers.image.description="Architecture linter for Go and Rust projects"
LABEL org.opencontainers.image.licenses="MIT"

# Copy binaries from builder stages
COPY --from=go-builder /out/archlint /usr/local/bin/archlint
COPY --from=rs-builder /out/archlint-rs /usr/local/bin/archlint-rs

# Install Python dependencies
RUN pip install --no-cache-dir \
    networkx>=3.0 \
    pyyaml>=6.0 \
    numpy>=1.24 \
    scipy>=1.10

# Copy validator package
COPY validator/ /opt/archlint/validator/

# Set PYTHONPATH so validator is importable as a module
ENV PYTHONPATH=/opt/archlint

WORKDIR /workspace

ENTRYPOINT ["archlint"]
