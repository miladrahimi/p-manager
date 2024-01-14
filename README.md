# Shadowsocks Manager

## Documentation

### Installation

A convenient method for installing Shadowsocks Manager involves cloning this repository onto the bridge server and executing it using Docker Compose.

``` shell
git clone https://github.com/miladrahimi/shadowsocks-manager.git
cd shadowsocks-manager
docker compose up -d
```

### Setting up Shadowsocks servers

To set up Shadowsocks servers on remote servers, you can execute the following shell commands on the server.
The final command will display the Shadowsocks server information, which you can input into the Shadowsocks Manager's "Servers" tab.

``` shell
# Install dependencies
apt-get -y update && apt-get -y upgrade && apt-get -y install wget curl vim git

# Install docker
wget -O install-docker.sh https://get.docker.com && \
  chmod +x install-docker.sh && ./install-docker.sh

# Install BBR
wget -N --no-check-certificate https://github.com/teddysun/across/raw/master/bbr.sh && \
  chmod +x bbr.sh && bash bbr.sh

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
