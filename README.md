# P-Manager

## Documentation

### Installation

1. Install the requirements

```shell
apt -y update && apt-get -y upgrade
apt -y install make wget curl vim git openssl cron
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

### Status and Logs

The application uses its directory name (default `p-manager`) in Systemd, allowing multiple instances to run on a single server.

To check the status of the application, execute the following command:

```shell
systemctl status p-manager
```

To view the application's standard outputs, execute the command below:

```shell
journalctl -f -u p-manager
```

The application logs will be stored in the following directory:

```shell
./storage/logs
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
