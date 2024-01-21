# Xray Manager

## Documentation

### Installation

```shell
# Install the requirements
apt-get -y update && apt-get -y upgrade
apt-get -y install make wget curl vim git openssl jq

# Install Docker
wget -O install-docker.sh https://get.docker.com
chmod +x install-docker.sh && ./install-docker.sh

# Install BBR
wget -N --no-check-certificate https://github.com/teddysun/across/raw/master/bbr.sh
chmod +x bbr.sh && bash bbr.sh
```

```shell
# Install Xray Manager
git clone https://github.com/miladrahimi/xray-manager.git
cd xray-manager
cp configs/main.json configs/main.local.json
docker compose up -d
# It runs on port 8080 by default.
```

### Configuration

```shell
configs/main.local.json
```

### Update

``` shell
docker compose pull
git pull
docker compose down
docker compose up -d
```

### Migrate

1. First set up the Xray Node on the foreign server.
2. Install and run Xray Manager on the bridge server with the default port (8080).
3. Visit Xray Manager in your browser:
    1. Open the "Servers" tab, and enter the Xray Node info (set a random port for `Ss Local Port`).
    2. Open the "Settings" tab, and import users from the old app.
    3. In the "Settings" tab, set `Shadowsocks Host` to the bridge server IP address.
    4. Open a user profile and check if its shadowsocks link works fine!
5. Run `docker compose down` for the `shadowsocks` (old) app.
6. Run `docker compose down` for the `outline-bridge-server` app.
7. Update the HTTP port to `80` in the `configs/main.local.json` file.
9. Run `docker compose restart` for Xray Manager.
10. Update the `Ss Local Port` in the "Servers" tab to the old shadowsocks port, then your users won't feel this change!
11. In the case of UI problems, use the full refresh option in your browser.

## Link

* https://github.com/miladrahimi/xray-node
