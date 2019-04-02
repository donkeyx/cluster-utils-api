# our base image
FROM node:alpine

ENV LANG en_AU.UTF-8
ENV LANGUAGE en_AU.UTF-8
ENV LC_ALL en_AU.UTF-8
ENV LC_CTYPE=en_AU.UTF-8
ENV TZ="Australia/Adelaide"

COPY .app.js /app/app.js

ENTRYPOINT ["node", "api.js"]

EXPOSE 80