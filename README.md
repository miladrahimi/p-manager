# P-Manager

P-Manager is a web-based proxy management panel built around Xray, designed with a minimal interface for maximum control.
It provides a unified platform to manage users, monitor traffic usage, add and configure nodes, and run as a proxy server or relay.
It supports Xray, SSH, and [paqet](https://github.com/hanselime/paqet) (raw-packet) based connection solutions.

## Documentation

### Installation

1- Install the requirements

```shell
apt-get -y update && apt-get -y upgrade
apt-get -y install make wget curl jq vim git openssl cron openssh-client
```

2- Install BBR (Optional)

```shell
echo "net.core.default_qdisc=fq" >> /etc/sysctl.conf
echo "net.ipv4.tcp_congestion_control=bbr" >> /etc/sysctl.conf
sysctl -p
```

3- Install P-Manager

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

* `Accounts`: Manage accounts and view their public profiles
* `Nodes`: Manage P-Nodes (remote nodes)
* `System`: Modify settings and view metrics
* `Exit`: Sign out of the admin panel

#### Automatic P-Nodes on Hetzner

Set a Hetzner Cloud project API token (read & write) in `System > Main Settings`, then use `+ Hetzner Node`
on the `Nodes` tab. P-Manager creates the cheapest in-stock x86 server in Germany (IPv4 only, newest Debian,
all SSH keys of the project), installs P-Node on it over SSH, and adds it as a node. Deleting such a node deletes
its Hetzner server as well. The SSH key of the host running P-Manager must already be one of the project's keys.

#### Remote CXH behind Cloudflare

Remote CXH (VLESS / XHTTP / TLS) puts P-Nodes behind Cloudflare, so clients dial a subdomain of your domain and never a
node IP. Put the domain on Cloudflare with SSL mode **Full (strict)**, create an API token scoped to that zone with
**Zone → Zone → Read**, **Zone → DNS → Edit** and **Zone → SSL and Certificates → Edit**, and enter the token, the
domain and a rotation interval in `System > Integrations`. Then enable `Remote CXH` in `System > Xray Settings` with
one of the HTTPS ports Cloudflare proxies (443, 2053, 2083, 2087, 2096, 8443). P-Manager issues an Origin CA
certificate, points a proxied random subdomain at every node, rotates it on the interval (the previous one keeps
working for one more interval) and adds the links to the accounts and subscriptions. P-Nodes need a P-Node build
that accepts inline certificates (2026-10 or newer).

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

### Uninstall

The command below uninstalls the P-Manager service, update cron job, and application directory.

```shell
make uninstall
```

### Status and Logs

The application service is named after its directory, with `p-manager` as the default in `systemd`.
It allows running multiple instances on a single server by placing the application in different directories.
A naming strategy for multiple instances could be using `p-manager-1` and `p-manager-2`, etc.

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

You can manually stop the application service,
replace the backup file with `./storage/database/data.json`,
and start the service again.

### Requirements

* Operating systems: Debian or Ubuntu
* Architecture: `amd64`
* RAM: 1 GB or more
* CPU: 1 Core or more

## Links

* [P-Node](https://github.com/miladrahimi/p-node)

## License

This project is governed by the terms of the [LICENSE](LICENSE.md).
