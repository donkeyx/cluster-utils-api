FROM golang:1.26 AS builder
WORKDIR /app

# Cache module downloads when only source changes
COPY go.mod go.sum ./
RUN go mod download

COPY . .
LABEL org.opencontainers.image.source=https://github.com/donkeyx/cluster-utils-api
LABEL maintainer="David Binney <donkeysoft@gmail.com>"

# Reproducible image build: locked modules only (no go get -u)
RUN make build

# no longer using musl dns moved to debian
FROM debian:stable-slim
WORKDIR /app
# ca-certificates required for HTTPS (proxy, OTEL OTLP, etc.)
RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates \
	&& rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/bin .
RUN ln -s /app/cu-api /usr/local/bin/cu-api; ln -s /app/cu-api /usr/local/bin/node; ln -s /app/cu-api /usr/local/bin/npm;
EXPOSE 8080
ENTRYPOINT [ "/app/cu-api" ]
