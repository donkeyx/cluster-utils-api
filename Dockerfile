FROM golang:1.21
WORKDIR /app
COPY . /app/

LABEL org.opencontainers.image.source=https://github.com/donkeyx/cluster-utils-api
LABEL maintainer="David Binney <donkeysoft@gmail.com>"

RUN make deps build

# no longer using musl dns moved to debian
FROM debian:stable-slim
WORKDIR /app
COPY --from=0 /app/bin .
#RUN ln -s /app/cu-api /usr/local/bin/cu-api; ln -s /app/cu-api /usr/local/bin/node; ln -s /app/cu-api /usr/local/bin/npm;
EXPOSE 8080
ENTRYPOINT [ "/app/cu-api" ]
