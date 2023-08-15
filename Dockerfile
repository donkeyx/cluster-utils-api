# # multi stage build, yo!
FROM golang:1.18.2
COPY . /app/
WORKDIR /app
RUN make deps build

# switched for potential shell
FROM debian-slim:latest
WORKDIR /app
COPY --from=0 /app/bin/cu-api /app/cu-api
ENTRYPOINT [ "/app/cu-api" ]
