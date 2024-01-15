# Shadowsocks Manager

## Documentation

### Installation

``` shell
git clone https://github.com/miladrahimi/shadowsocks-manager.git
cd shadowsocks-manager
cp configs/main.json configs/main.local.json
docker compose up -d
# It runs on port 8080 by default.
```

### Configuration

You can change the configurations in the `configs/main.local.json` file.

### Update

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

### Migrate from the old app

1. First set up the foreign Shadowsocks server (explained in the section above).
2. Install and run `shadowsocks-manager` on the bridge server with the default port (8080).
3. Visit `shadowsocks-manager` in your browser:
    1. Open the "Servers" tab, and enter the foreign Shadowsocks server
    2. Open the "Settings" tab, and import users from the old app.
    3. In the "Settings" tab, set `Shadowsocks Host` to the bridge server IP address.
    4. Open a user profile and check if its shadowsocks link works fine!
5. Run `docker compose down` for the `shadowsocks` (old) app.
6. Run `docker compose down` for the `outline-bridge-server` app.
7. Update the HTTP port to 80 in the `configs/main.local.json` file.
9. Run `docker compose down` and `docker compose up -d` for the `shadowsocks-manager` app.
10. Update the `Shadowsocks Port` in the "Settings" tab to the old port, then your users won't understand this change!
