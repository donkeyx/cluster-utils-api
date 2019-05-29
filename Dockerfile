# our base image
FROM node:alpine

ENV LANG en_AU.UTF-8 \
    LANGUAGE en_AU.UTF-8 \
    LC_ALL en_AU.UTF-8 \
    LC_CTYPE=en_AU.UTF-8 \
    TZ="Australia/Adelaide"

WORKDIR /usr/src/app

COPY package*.json ./
RUN npm ci

COPY server.js ./

ENTRYPOINT ["npm", "start"]
EXPOSE 8080