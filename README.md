# Shadowsocks Manager

## Documentation

### Installation

The convenient method for installing Shadowsocks Manager is cloning this repository onto the bridge server and executing it using Docker Compose.

``` shell
git clone https://github.com/miladrahimi/shadowsocks-manager.git
cd shadowsocks-manager
cp configs/main.json configs/main.local.json
docker compose up -d
```
It typically runs on port 8080, but you can adjust this setting in the `configs/main.local.json` file.

### Configuration

It is recommended to create a `configs/main.local.json` file based on `configs/main.json` before running the app.
If you followed the installation instructions above, you have already completed this step.
The application uses `configs/main.local.json` and since it is set to be ignored by Git, updating the app becomes easier.

### Update

To update the application, execute the following commands in the app directory:

``` shell
docker compose pull
git pull
docker compose down
docker compose up -d
```

### Setting up Shadowsocks servers

To set up Shadowsocks servers on foreign servers, you can execute the following shell commands on the server.
The final command will display the Shadowsocks server information, which you can enter into the Shadowsocks Manager's "Servers" tab.

``` shell
# Install dependencies
apt-get -y update && apt-get -y upgrade && apt-get -y install wget curl vim git openssl

# Install docker
wget -O install-docker.sh https://get.docker.com && chmod +x install-docker.sh && ./install-docker.sh

# Install BBR
wget -N --no-check-certificate https://github.com/teddysun/across/raw/master/bbr.sh && chmod +x bbr.sh && bash bbr.sh

# Install NGINX
docker run --name my_nginx -p 80:80 -d nginx

# Generate shadowsocks configurations
PASSWORD=$(openssl rand -base64 16 | tr -dc 'a-zA-Z0-9') && echo '{
    "server": "0.0.0.0",
    "server_port": 1919,
    "password": "'"$PASSWORD"'",
    "method": "chacha20-ietf-poly1305"
}' > config.json

# Run shadowsocks server
docker run --name ssserver-rust \
  --restart always \
  -p 1919:1919/tcp \
  -p 1919:1919/udp \
  -v ./config.json:/etc/shadowsocks-rust/config.json \
  -dit ghcr.io/shadowsocks/ssserver-rust:latest

# Show shadowsocks configurations
cat config.json
```
