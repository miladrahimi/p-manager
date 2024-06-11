# P-Manager

## Documentation

### Installation

1. Install the requirements

```shell
apt-get -y update && apt-get -y upgrade
apt-get -y install make wget curl vim git openssl cron
```

2. Install BBR (Optional)

```shell
echo "net.core.default_qdisc=fq" >> /etc/sysctl.conf
echo "net.ipv4.tcp_congestion_control=bbr" >> /etc/sysctl.conf
sysctl -p
```

3. Install P-Manager

```shell
git clone https://github.com/miladrahimi/p-manager.git
cd p-manager
make setup
```

### Admin Panel

Access the admin panel at the default port `8080`.

#### Default credentials

* Username: `admin`
* Password: `password`

#### Tabs

* `Users`: Manage users and view their public profiles
* `Servers`: Manage P-Nodes
* `Settings`: Modify general settings
* `Exit`: Sign out of the admin panel

### Configuration

You can customize the web panel port and additional settings by modifying the configuration file found at:

```shell
configs/main.json
```

It requires `systemctl restart p-manager` to apply changes.

### Update

Automatic updates are set up through cron jobs by default.
For earlier updates, run the command below:

``` shell
make update
```

### Requirements

 * Operating systems: Debian or Ubuntu
 * Architecture: `amd64`
 * RAM: 1 GB or more
 * CPU: 1 Core or more

## Links

* [P-Node](https://github.com/miladrahimi/p-node)

## License

This project is governed by the terms of the [LICENSE](LICENSE.md).
