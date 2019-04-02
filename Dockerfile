# our base image
FROM node:alpine

ENV LANG en_AU.UTF-8
ENV LANGUAGE en_AU.UTF-8
ENV LC_ALL en_AU.UTF-8
ENV LC_CTYPE=en_AU.UTF-8
ENV TZ="Australia/Adelaide"

WORKDIR /usr/src/app

COPY package*.json server.js ./

RUN npm ci

ENTRYPOINT ["npm", "start"]

EXPOSE 8080