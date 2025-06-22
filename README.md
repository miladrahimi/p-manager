# P-Manager

## Documentation

### Installation

1. Install the requirements

```shell
apt-get -y update && apt-get -y upgrade
apt-get -y install make wget curl jq vim git openssl cron
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

The application service is named after its directory, with `p-manager` as the default in `systemd`.
It allows running multiple instances on a single server by placing the application in different directories with different names (like `p-manager-2` and `p-manager-3`).

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

### Backups and Recovery

The application performs hourly database backups and saves them to the path below.

```
./storage/database/backup-%weekday-%hour.json
```

It creates a total of 168 backup files (7 days x 24 hours), covering a full week.
Backups older than one week are unavailable due to the file path structure.

To restore the most recent backup, execute the following command:

```
make recover
```

You can manually stop the application service, replace the backup file with `./storage/database/app.json`, and start the service again.

### Requirements

* Operating systems: Debian or Ubuntu
* Architecture: `amd64`
* RAM: 1 GB or more
* CPU: 1 Core or more

## Links

* [P-Node](https://github.com/miladrahimi/p-node)

## License

This project is governed by the terms of the [LICENSE](LICENSE.md).
