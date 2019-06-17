# our base image
FROM node:alpine

RUN apk add --no-cache curl

ENV LANG en_AU.UTF-8 \
    LANGUAGE en_AU.UTF-8 \
    LC_ALL en_AU.UTF-8 \
    LC_CTYPE=en_AU.UTF-8 \
    TZ="Australia/Adelaide"

WORKDIR /usr/src/app

HEALTHCHECK --interval=5s --timeout=3s --start-period=5s \
  CMD curl --fail http://127.0.0.1:80/healthz || exit 1

COPY package*.json ./
RUN npm ci

COPY server.js ./

ENTRYPOINT ["npm", "start"]
EXPOSE 8080
