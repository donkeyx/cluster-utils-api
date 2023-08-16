# # multi stage build, yo!
FROM golang:1.21
WORKDIR /app
COPY . /app/

LABEL org.opencontainers.image.source https://github.com/donkeyx/cluster-utils-api
LABEL maintainer="David Binney <donkeysoft@gmail.com>"

RUN make deps build

# no longer using musl dns moved to debian
FROM debian:stable-slim
WORKDIR /app
ENV \
    LANG en_AU.UTF-8 \
    LANGUAGE en_AU.UTF-8 \
    LC_ALL en_AU.UTF-8 \
    LC_CTYPE=en_AU.UTF-8 \
    TZ="Australia/Adelaide"

COPY --from=0 /app/bin/cu-api /app/cu-api
ENTRYPOINT [ "./cu-api" ]
