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

### Admin Panel

Access the admin panel at the default port `8080`.

#### Default credentials

* Username: `admin`
* Password: `password`

#### Tabs

* Users: Manage users and view their public profiles
* Servers: Manage [P-Nodes](https://github.com/miladrahimi/p-node)
* Settings: Modify the general settings
* Exit: Sign out of the web panel

### Configuration

You can customize the web panel port and additional settings by modifying the local configuration file found at:

```shell
configs/main.local.json
```

It requires `docker compose restart` to apply changes.

### Update

Automatic updates are set up through cron jobs by default.
For earlier updates, run the command below:

``` shell
make update
```

### Requirements

 * Operating systems: Debian or Ubuntu
 * RAM: 1 GB or more
 * CPU: 1 Core or more

## Links

* https://github.com/miladrahimi/p-node

## License

This project is governed by the terms of the [LICENSE](LICENSE.md).
