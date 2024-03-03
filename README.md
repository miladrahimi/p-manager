# Xray Manager

## Documentation

### Installation

```shell
# Install the requirements
apt-get -y update && apt-get -y upgrade
apt-get -y install make wget curl vim git openssl
```

```shell
# Install Docker
wget -O install-docker.sh https://get.docker.com
chmod +x install-docker.sh && ./install-docker.sh && rm install-docker.sh
```

```shell
# Install BBR
sudo sh -c 'echo "net.core.default_qdisc=fq" >> /etc/sysctl.conf'
sudo sh -c 'echo "net.ipv4.tcp_congestion_control=bbr" >> /etc/sysctl.conf'
sudo sysctl -p
```

```shell
# Install Xray Manager
git clone https://github.com/miladrahimi/xray-manager.git
cd xray-manager
make setup
docker compose up -d
```

### Web Panel

Access the web panel at the default port 8080. Log in using the username `admin` and password `password`.
* In the `Users` tab, you can manage users and view their public profiles.
* In the `Servers` tab, you can add each [Xray Node](https://github.com/miladrahimi/xray-node) by specifying their Host (IP), HTTP Port, and Token.
* In the `Settings` tab, you can modify the Admin password, the Host, the Ports, etc.

### Configuration

```shell
# Modify web panel port and other configurations.
# It requires `docker compose restart` to apply changes.
configs/main.local.json
```

### Update

``` shell
# Execute this each time a new version is released
make update
```

### Requirements

 * Operating system: Debian or Ubuntu
 * RAM: 1 GB
 * CPU: 1 Core

## Links

* https://github.com/miladrahimi/xray-node

## License

This project is governed by the terms of the [LICENSE](LICENSE.md) license.
Users are only allowed to use the Project with explicit permission from the creator.
