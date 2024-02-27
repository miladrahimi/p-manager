# syntax=docker/dockerfile:1

## Build
FROM ghcr.io/getimages/golang:1.21.6-bookworm AS build

WORKDIR /app
COPY . .

RUN go mod tidy
RUN go build -o xray-manager

RUN tar -zcf web.tar.gz web

## Deploy
FROM ghcr.io/getimages/debian:bookworm-slim

WORKDIR /app

COPY --from=build /app/xray-manager xray-manager
COPY --from=build /app/assets/ed25519_public_key.txt assets/ed25519_public_key.txt
COPY --from=build /app/configs/main.json configs/main.json
COPY --from=build /app/storage/.gitignore storage/.gitignore
COPY --from=build /app/third_party/xray-linux-64/xray third_party/xray-linux-64/xray
COPY --from=build /app/web.tar.gz web.tar.gz

RUN tar -xvf web.tar.gz
RUN rm web.tar.gz

EXPOSE 8080

ENTRYPOINT ["./xray-manager", "start"]
