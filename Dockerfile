# syntax=docker/dockerfile:1

## Build
FROM ghcr.io/getimages/golang:1.21.0-bullseye AS build

WORKDIR /app

COPY . .
RUN go mod tidy
RUN go build -o ssm

RUN ./third_party/install-xray.sh

RUN tar -zcf third_party.tar.gz third_party
RUN tar -zcf web.tar.gz web

## Deploy
FROM ghcr.io/getimages/debian:bullseye-slim

WORKDIR /app

COPY --from=build /app/shadowsocks shadowsocks
COPY --from=build /app/configs/main.json configs/main.json
COPY --from=build /app/storage/.gitignore storage/.gitignore
COPY --from=build /app/third_party.tar.gz third_party.tar.gz
COPY --from=build /app/web.tar.gz web.tar.gz

RUN tar -xvf third_party.tar.gz
RUN tar -xvf web.tar.gz
RUN rm third_party.tar.gz
RUN rm web.tar.gz

EXPOSE 80

ENTRYPOINT ["./ssm", "start"]
