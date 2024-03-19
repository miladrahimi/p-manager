# P-Manager

## Documentation

### Installation

```shell
# Install the requirements
apt-get -y update && apt-get -y upgrade
apt-get -y install make wget curl vim git openssl cron
```

```shell
# Install Docker
wget -O install-docker.sh https://get.docker.com
chmod +x install-docker.sh && ./install-docker.sh && rm -f install-docker.sh
```

```shell
# Install BBR
sudo sh -c 'echo "net.core.default_qdisc=fq" >> /etc/sysctl.conf'
sudo sh -c 'echo "net.ipv4.tcp_congestion_control=bbr" >> /etc/sysctl.conf'
sudo sysctl -p
```

```shell
# Install P-Manager
git clone https://github.com/miladrahimi/p-manager.git
cd p-manager
make setup
docker compose up -d
```

### Web Panel

Access the web panel at the default port `8080`.
Sign in using the username `admin` and password `password`.
* In the `Users` tab, you can manage users and view their public profiles.
* In the `Servers` tab, you can add any number of [P-Nodes](https://github.com/miladrahimi/p-node).
* In the `Settings` tab, you can modify the general settings.

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

 * Operating system: Linux (Debian or Ubuntu)
 * RAM: 1 GB or more
 * CPU: 1 Core or more

## Links

* https://github.com/miladrahimi/p-node

## License

This project is governed by the terms of the [LICENSE](LICENSE.md).
