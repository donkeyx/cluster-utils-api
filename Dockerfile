FROM golang:1.26 AS builder
WORKDIR /app
COPY . /app/

LABEL org.opencontainers.image.source=https://github.com/donkeyx/cluster-utils-api
LABEL maintainer="David Binney <donkeysoft@gmail.com>"

# Reproducible image build: use locked modules, do not go get -u
RUN make deps build

# no longer using musl dns moved to debian
FROM debian:stable-slim
WORKDIR /app
COPY --from=builder /app/bin .
RUN ln -s /app/cu-api /usr/local/bin/cu-api; ln -s /app/cu-api /usr/local/bin/node; ln -s /app/cu-api /usr/local/bin/npm;
EXPOSE 8080
ENTRYPOINT [ "/app/cu-api" ]
