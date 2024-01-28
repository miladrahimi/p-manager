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
make update
```

## Link

* https://github.com/miladrahimi/xray-node
