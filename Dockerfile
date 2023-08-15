# # multi stage build, yo!
FROM golang:1.21
COPY . /app/
WORKDIR /app

LABEL org.opencontainers.image.source https://github.com/donkeyx/cluster-utils-api
LABEL maintainer="David Binney <donkeysoft@gmail.com>"

RUN make deps build

# switched for potential shell
FROM debian:stable-slim
WORKDIR /app
COPY --from=0 /app/bin/cu-api /app/cu-api
ENTRYPOINT [ "./cu-api" ]
