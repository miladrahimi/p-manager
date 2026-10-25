# CLAUDE.md

## Project summary
P-Manager is a Go service that manages users (accounts) and remote P-Nodes, generates Xray configs, and serves a static admin UI.
It stores state in JSON files under `storage/` and syncs configs and stats between the local Xray instance and P-Nodes.

## Runtime Flow
- `main.go` → `cmd/root.go` (Cobra) → `cmd/serve.go` (`serve` command).
- `internal/app.New()`: Builds config, logger, database, http client, xray, ssh client/pool, composer, coordinator, and http server.
- `App.Start()`: Initializes the database, runs the coordinator, and starts the http server.
- `App.Wait()` blocks on the app context; `App.Close()` shuts everything down (http server, xray, ssh pool, saves database, closes logger).
- Signals (`SIGINT`, `SIGTERM`) are handled in `internal/app/app.go` to cancel the context and stop gracefully.

## Tech stack
- Main: Go 1.25.x, Cobra CLI, Echo v4 HTTP server
- UI: static HTML + Alpine.js (behavior) + Tailwind CSS v4 (styling), no build toolchain beyond the Tailwind CLI
- Reuses several packages from the P-Node repo (`github.com/miladrahimi/p-node`): `pkg/database`, `pkg/logger`, `pkg/xray`, `pkg/paqet`, `pkg/http/client`, `pkg/http/logs`, `pkg/worker`, and `pkg/util` for filesystem/port helpers (`pkg/util` here holds only manager-specific helpers).

## Packages
- `cmd`: Cobra commands (`serve`)
- `configs`: Configuration files (`main.defaults.json`, optional `main.json`, `main.example.json`)
- `internal/app`: App lifecycle orchestration
- `internal/composer`: Xray configuration generator for local and remote Xray instances (subpackages `vless`, `socks`; also `link_composer.go` for share links)
- `internal/config`: Configuration loader, constants (`config.go`), and filesystem paths (`paths.go`)
- `internal/coordinator`: Periodic sync between current state, local Xray process, and P-Nodes (`config_syncer`, `stats_syncer`, `ssh_syncer`, `paqet_syncer`, `state`)
- `internal/data`: Database schema, models, and default values
- `internal/http/server`: Echo server setup and routing
- `internal/http/handlers`: HTTP handlers (`handlers/api` for JSON APIs, plus the static `account` page handler)
- `internal/provisioner`: Creates a Hetzner server, installs P-Node on it over SSH, and registers it as a node (in-memory jobs; also deletes the server when such a node is removed)
- `pkg/hetzner`: Minimal Hetzner Cloud API client (ssh keys, locations, server types, images, servers)
- `pkg/ssh`: SSH client (SOCKS check/tunnels, remote command `Run`, known_hosts cleanup), connection/proxy config, process management, and connection pool. Authentication is always key-based (batch mode)
- `pkg/util`: Generic utils
- `scripts`: Scripts for project and server setup
- `storage`: Application data storage directory (database, logs, generated xray config)
- `third_party`: Third-party binaries and libraries (Xray and paqet binaries; Linux amd64 is tracked, macOS/arm64 builds come from `make local-setup` / `make e2e-docker` and are ignored)
- `web`: Static admin web UI files. Alpine.js is vendored under `assets/third_party`; `assets/js/app.js` holds the shared fetch-based API client and helpers, including the toast store and the in-page confirm dialog (`confirmDialog(...)` + `$store.dialog`; never use native `alert`/`confirm`, each page renders the shared dialog markup next to its toasts). Styling is Tailwind: edit `assets/css/app.src.css`, then run `make web` to regenerate the committed `assets/css/app.css`.

## Database
- File-backed JSON store.
- Driver is the generic `github.com/miladrahimi/p-node/pkg/database` package, used as `database.Database[data.Data]`.
- Schema is defined in `internal/data/data.go` (`Data` struct: accounts, nodes, stats, main settings, xray settings).
- The database directory is `storage/database`, returned by `config.DatabaseDirectory(root)` in `internal/config/paths.go`. The driver owns the data/backup file naming inside that directory.
- `database.New(dir, data.Default())` creates it; `db.Init()` loads, `db.Save()` persists.

## Configuration
- Loaded in `internal/config/config.go` via `config.New(root)`.
- Defaults come from `configs/main.defaults.json`; if `configs/main.json` exists it overrides defaults and is re-written (pretty-printed) on load.
- Validated with `go-playground/validator`.
- Build/version constants live in `config.go`: `AppName`, `AppVersion`, `CoreName`, `MaxAccountsCount`, `DefaultNodeSni`, `DefaultManagerSni`.

## HTTP APIs
- Server and routes are wired in `internal/http/server`.
- API handlers live in `internal/http/handlers/api`; requests and responses are JSON.
- Routes are grouped by audience under `/api` (`internal/http/server/server.go`):
  - `/api/admin/*` — admin panel; guarded by `Server.authorizeAdmin` (the admin password `admin_password`, `internal/data/main_settings.go`), except `POST /api/admin/sign-in` which issues the token.
  - `/api/user/*` — account-holder APIs (account view, links renew); no auth, reached via the account link.
  - `/api/admin/nodes/hetzner` — Hetzner provisioning: `GET` lists jobs (+ whether a token is set), `POST` starts one (body `manager_url`, the panel origin, used to run `set-manager` on the node), `DELETE /:jobId` dismisses a failed job.
  - `/api/admin/logs` — log viewer: `GET` lists the `*.log` files under `storage/logs` (`config.LogsDirectory`); `GET /:name?lines=N` streams one file as chunked `text/plain` (`tail -f`: the last N lines, then every appended byte, polling every 500ms; truncated/replaced files restart from their start) until the client disconnects. The admin page (`web/admin-logs.html`) reads it with `fetch` + `ReadableStream` (not `EventSource`, which cannot send the Bearer header), parses zap JSON lines client-side, and reconnects on drop. The index/stream handlers are P-Node's shared `pkg/http/logs`.
  - `/api/admin/nodes/:id/logs` — node logs: `GET` lists and `GET /:name?lines=N` streams a P-Node's log files by proxying its own `GET /logs` and `GET /logs/:name` (authenticated with the node's HTTP token, so the node needs `HttpPort`/push). The logs page has a source select: Manager or any node with an HTTP port.
  - `/api/node/*` — node-facing APIs (`GET /api/node/:id/config`); guarded by `Server.authorizeNode` against that node's own `PullToken` (`Node.PullToken`, generated at creation). A node token works only for its own node and never for admin APIs, so the pull command carries no admin credentials.
- Authentication is `Authorization: Bearer <token>`.
- `GET /subscription/:proxyId` (public) always serves an account's subscription (all links, base64). The `SubscriptionEnabled` main setting (off by default) only controls whether the account page shows that link; when off, the page shows just the individual server links and the setup guide uses its per-link wording (`steps` entries with `subscription`/`links` variants in `web/account.html`).

## Major Dependencies
- `github.com/xtls/xray-core`: Interaction with the running Xray process
- `github.com/labstack/echo/v4`: HTTP server and router
- `github.com/spf13/cobra`: CLI framework
- `github.com/go-playground/validator/v10`: Validating config and data models
- `github.com/cockroachdb/errors`: Errors and stacktraces
- `go.uber.org/zap`: Logging (wrapped by p-node's `pkg/logger`)
- `github.com/miladrahimi/p-node`: Shared database, logger, xray, and http client packages

## Build & Run
- `make local-serve` — run locally (`go run main.go serve`)
- `make build` — cross-compile the Linux amd64 `p-manager` binary
- `make e2e` — end-to-end test (`e2e/`, build tag `e2e`): builds P-Manager and P-Node, runs both as processes in temp dirs, and fetches a local HTTP target through every proxy method via an Xray client; needs the local Xray binary (`make local-setup`) and `../p-node` (override with `P_NODE_DIR`); set `E2E_SSH=1` to include Relay RR2SSH (needs passwordless ssh to 127.0.0.1); set `E2E_PQ=1` to include Relay RR2PQ (needs root on Linux: the test creates a veth pair `pq0`/`pq1` for paqet); set `E2E_CF_TOKEN` + `E2E_CF_DOMAIN` to run Remote CXH against the real Cloudflare API on that zone (certificate revoked and records deleted at the end) instead of the in-test fake; `CF_TOKEN` + `CF_DOMAIN` likewise run `pkg/cloudflare`'s `TestRealApi`
- `make e2e-docker` — the same e2e with `E2E_PQ=1` inside a privileged Linux container (`e2e/Dockerfile`, `scripts/e2e-docker.sh`); downloads the Linux binaries for the container architecture into `third_party` on first use. This is the only way to run RR2PQ end to end on a Mac: paqet needs root and an Ethernet interface (no loopback)
- `make web` — rebuild the admin UI stylesheet (`web/assets/css/app.css`) via the Tailwind standalone CLI (downloaded to `bin/` on first use); commit the regenerated CSS
- `make local-setup` / `make setup` — local / server setup scripts
- `make clean` / `make fresh` — remove logs / wipe storage state (`app`, `database`, `logs`)
- `make update` — **destructive**: `git fetch` + `git reset --hard` + `git clean -fd` + `git pull` + setup

## Key Behavior Notes / Gotchas
- Admin panel username is `admin`; the password is the `AdminPassword` setting (`internal/data/main_settings.go`).
- Default admin password is `password` (`defaultAdminPassword`); change it after first login. Validation requires 8–32 chars.
- `make update` is destructive (`git reset --hard` + `git clean -fd`). On a server with a slow route to GitHub
  (Iran: ~14 KB/s, the 27 MB binary blob gets cut off) pull through a node first: `ssh -fN -D 127.0.0.1:1089
  root@<node>` + `git -c http.proxy=socks5h://127.0.0.1:1089 pull` (README → Update). A `make update` killed
  mid-pull leaves an orphaned `git merge` holding `.git/index.lock`; stop it with `pkill -x git`, never
  `pkill -f "make update"` from a remote command line that itself contains those words.
- Relay RR2SSH tunnels to one node start `sshStartStagger` (300 ms) apart: sshd refuses connections above
  `MaxStartups` (10 by default) while they are still authenticating, so a burst of 16 lost three on every restart
  (they came back after the 2 s retry, but it filled the log with `ssh: process exited 255`).
- Xray zeroes its counters on restart, and every manager restart pushes node configs with a new
  `_metadata.updatedAt` that makes P-Node restart Xray. So the config syncer reads the counters first:
  `beforeReconfigure` (local `loadLocalStats`) and `beforePush` (`pullStatsBeforePush`, reachable nodes only, when
  Remote RR or CXH is on).
- **Usage accounting** (`coordinator/stats_aggregate.go`, pure functions with tests): `aggregateLocalStats` reads the
  manager's counters — accounts from `user>>>` (except reverse bridge ids, which are attributed to their node), nodes
  from the `relay-*` outbounds plus the bridge users, total and per-method from the client inbounds `direct-rr`,
  `relay-rr2rr`, `relay-rr2ssh`, `relay-rr2pq`, `reverse-rr` (counted once; `reverse-rr-tunnel` carries the same
  bytes). `aggregateNodeStats` reads a node's counters — accounts from `user>>>`, node and per-method from the client
  inbounds `remote-rr` and `remote-cxh`, minus the relay client's bytes on a shared port. Per-method totals live in
  `Stats.MethodUsageBytes` (keyed by inbound tag, reset together with the total via `PATCH /api/admin/stats`), served
  as `method_usage_bytes` and shown as "Usage by method" on the System page.
- The built `p-manager` binary is tracked and should be committed when updated.
- `go.mod` uses `replace github.com/miladrahimi/p-node => ../p-node` to develop against a local P-Node checkout.
- Use Java-style camelCase for namings (`UserId` instead of `userID`, `clientId` instead of `clientID`, etc.).
- Hetzner provisioning (`internal/provisioner`): needs `MainSettings.HetznerToken`. Picks the cheapest non-deprecated x86 server type in stock at a `DE` location (tie → more memory), the newest non-deprecated `debian-*` system image, IPv4 only, with all SSH keys of the Hetzner project injected (the manager host's key must already be one of them; nothing is generated or uploaded). Installs P-Node with its one-line installer as root (waits for cloud-init/apt locks), reads `http_port`/`http_token` from `/root/p-node-1/storage/database/data.json`, adds the node with `Provider=hetzner` + `ProviderServerId` and fills `Node.Details` (free-form `map[string]string` shown in the node details modal: provider, server, type, specs, location, image, price, IP, created), then runs `set-manager` best-effort. One job runs at a time; on failure the created server is deleted again. `NodesDelete` deletes the Hetzner server first and keeps the node if that fails.
- `Node` sync flags `SshEnabled`/`PushEnabled` default on via `NewNode`. There is no DB migration layer (backward compatibility with pre-flag databases is intentionally not supported), so the driver's `json.Unmarshal` leaves keys the file omits at their zero value — an old database would read those flags back as `false`; recreate/re-add nodes rather than relying on an upgrade path. Pulling is intentionally NOT a flag: a P-Node pulls only after its setup command is run on the node, so the pull status is derived from `PulledAt` (`pullStatus`) and shown read-only in the UI — there is nothing manager-side to enable/disable.

## Xray Proxy
Xray is a proxy platform which can run proxy servers with different protocols like Shadowsocks, VMess, VLess,
Socks, etc. It supports chains of proxies. The high-level flow is:

```
[ Client ] -> [ Xray on Server 1 ] -> [ Xray on Server 2 ] -> Internet
```

Each xray node receives traffic on its inbound port and forwards it to the next node in the chain.
Based on the routing rules and balancers it forwards traffic to the appropriate outbound.

Xray provides a reverse proxy feature that allows a connection to be initiated from the next node back to the previous
node. This is particularly useful when the previous node is behind a firewall (such as the GFW) that restricts outbound
connections to the network where the next node resides.

## SSH Proxy
SSH Proxy is the same SOCKS proxy provided by the SSH tool. The provided SOCKS proxy is used as an outbound for Xray.

## paqet (PQ)
[paqet](https://github.com/hanselime/paqet) is a raw-packet tunnel: it crafts TCP packets itself (pcap, no handshake, no
kernel connection tracking) and runs KCP + smux inside them. Its client exposes a local SOCKS5 proxy, which P-Manager
uses as an Xray outbound exactly like the SSH SOCKS proxies. P-Node's `pkg/paqet` runs the binary (either role) as a
supervised child process from a generated YAML config, detects the network it needs (default-route interface, its IPv4,
gateway MAC; overridable via the `paqet` block in `configs/main.json`), and on Linux adds the iptables NOTRACK/RST-drop
rules a paqet server needs. Both ends must run as root (pcap); the systemd services do.

## Supported Methods
Method names carry the transport: **RR** is VLESS over Raw (TCP) with Reality and the `xtls-rprx-vision` flow,
dialled at a real IP; **CXH** is VLESS over XHTTP with TLS behind a CDN (Cloudflare), dialled at a CDN subdomain,
no flow (vision needs raw). **SSH** and **PQ** (paqet) name the hop between the manager and a node.

### Direct RR
P-Manager works as a proxy-server.
It accepts direct VLESS Reality Raw requests from clients and forwards them to the Internet.
```
[ Client ] -(VLESS Reality Raw)-> [ P-Manager ] -> Internet
```

### Remote RR
P-Nodes work as proxy-servers.
They directly accept VLESS Reality Raw requests from clients and forward them to the Internet.
```
[ Client ] -(VLESS Reality Raw)-> [ P-Node ] -> Internet
```

### Relay RR2RR
P-Manager works as a relay server.
It accepts VLESS Reality Raw requests from clients and forwards them to the P-Nodes.
```
[ Client ] -(VLESS Reality Raw)-> [ P-Manager ] -(VLESS Reality Raw)-> [ P-Node ] -> Internet
```

`RelayRr2RrNodePort` may equal `RemoteRrPort` (`XraySettings.NodeRrShared`): the node then serves accounts and the
manager's relay client from the single `remote-rr` inbound, so one port (e.g. 443) carries both methods. The stats
syncer subtracts the relay client's user traffic from that inbound so the node usage is not counted twice.

### Relay RR2SSH
P-Manager works as a relay server.
It starts SSH connections to P-Nodes and provides SOCKS proxies.
It also accepts VLESS Reality Raw requests from clients and forwards them to the SOCKS proxies.
```
[ Client ] -(VLESS Reality Raw)-> [ P-Manager ] -(SOCKS)-> [ SSH to P-Node ] -> Internet
```

### Relay RR2PQ
P-Manager works as a relay server over paqet.
It runs one paqet client per P-Node (local SOCKS5 proxy, raw-packet KCP tunnel to the node's paqet server) and accepts
VLESS Reality Raw requests from clients, balancing them over those SOCKS proxies.
```
[ Client ] -(VLESS Reality Raw)-> [ P-Manager ] -(SOCKS)-> [ paqet client ] ~(raw TCP/KCP)~> [ paqet server on P-Node ] -> Internet
```
Enabled when `XraySettings.RelayRr2PqPort` (manager, clients) and `XraySettings.RelayRr2PqNodePort` (node, paqet server)
are set. The per-node shared key is a stable `util.StableUuid` (`composer.RelayRr2PqKey`), so both sides derive it. The
node learns its server port and key from the `_paqet` field carried inside the pushed/pulled Xray config
(`component.Paqet`; Xray ignores the key). Manager side: `coordinator/paqet_syncer.go` keeps the clients running
(`storage/app/paqet-<nodeId>.yaml`, log `storage/logs/paqet.log`), `State.PaqetPortsByNode` feeds the composer's
`relay-rr2pq` inbound + balancer over `relay-rr2pq-<nodeId>` SOCKS outbounds. Node side: `coordinator.Apply` reconfigures
Xray and the paqet server together (`storage/app/paqet.yaml`). KCP MTU is pinned to 1200 (`paqet.Mtu`) because Iranian
carriers fragment larger raw packets.

### Remote CXH
P-Nodes work as proxy-servers behind Cloudflare. Clients dial a proxied subdomain of the CDN domain over
VLESS XHTTP TLS; the Cloudflare edge terminates the client TLS and dials the node on the same port with a
Cloudflare Origin CA certificate the manager issued. Clients never see a node IP, and the inbound accepts any
`Host`, so a new subdomain needs no config push.
```
[ Client ] -(VLESS XHTTP TLS)-> [ Cloudflare ] -(VLESS XHTTP TLS)-> [ P-Node ] -> Internet
```
Enabled when `XraySettings.RemoteCxhPort` (one of `data.CloudflareHttpsPorts`: 443, 2053, 2083, 2087, 2096, 8443,
the only ports the edge proxies; validated in `XraySettingsUpdate`, distinct from the other node ports) and
`MainSettings.CloudflareToken` + `MainSettings.CdnDomain` (Integrations) are set, and the manager has issued the
certificate. The token needs Zone Read, DNS Edit and SSL and Certificates Edit on that zone; the zone's SSL mode must
be Full (strict). State the manager keeps itself lives in `data.Cdn` (`cxh_path`, zone id, certificate id + PEM
certificate and key, `records`): `coordinator/cxh_syncer.go` (`syncCdn`, every 20s, `pkg/cloudflare`) looks the zone
up, issues one 15-year Origin ECC certificate for `domain` + `*.domain` (`cloudflare.GenerateCsr`), creates a proxied
record on a random 6-character subdomain (`util.RandomLabel(cdnLabelLength)`) per node (A for an IP host, CNAME otherwise), rotates it every
`MainSettings.CdnRotationHours` (0 = never) keeping the newest `cdnRecordsPerNode` (2) so older clients survive one
interval, and deletes records of gone or re-hosted nodes. Turning the method off deletes the records and keeps the
certificate; changing the domain revokes the certificate and starts over (the path is kept). Cleanup forgets only
what Cloudflare confirmed deleted: failed deletes/revokes stay stored and are retried on the next run, a domain
switch waits until the old domain is clean, and without a token the state is kept for when one is set again. A certificate or path
change triggers a node push; a new record does not. The composer adds the `remote-cxh` inbound
(`vless.MakeCxhInbound`, certificate inline as PEM lines, `xhttpSettings.mode: auto`) and one link per node on its newest record
(`Cdn.NewestNodeRecord`; the previous subdomain keeps working but is no longer handed out) (`remote-cxh-<subdomain>@<host>`: `type=xhttp&security=tls&mode=packet-up&alpn=h2&fp=chrome`, SNI and `host` = the
subdomain). Node usage counts the `remote-cxh` inbound like `remote-rr`, and node stats are pulled whenever Remote RR or
Remote CXH is on (`nodeStatsEnabled`; before CXH the pull was gated on Remote RR alone, so a CXH-only setup counted
nothing). The e2e runs it against a fake Cloudflare
(`configs/main.json` → `cloudflare.base_url`) that signs the CSR with a throwaway CA; the test client pins the leaf
(`pinnedPeerCertSha256`; `allowInsecure` is gone in Xray 26) and dials the node in place of the edge.

### Reverse RR
P-Manager is the reverse portal; each P-Node is a bridge that dials out to it, so P-Nodes work without a public/reachable inbound (only the P-Manager host must be reachable, which nodes learn from the pushed/pulled config). Accounts hit the same manager port and the portal load-balances them across all connected nodes (nodes share the `reverse-rr` domain).
```
[ Client ] -(VLESS Reality Raw)-> [ P-Manager ] ↪ [ P-Node dials back ] -> Internet
```
Config (`internal/composer/config_composer.go`): manager adds a `reverse-rr` inbound on `ReverseRrManagerPort` (accounts with vision flow) and a `reverse-rr-tunnel` inbound on `ReverseRrPortalPort` (one no-flow bridge client per node), both routed to the `reverse-rr-portal`; each node adds a `reverse-rr-bridge` + a no-flow `reverse-rr-tunnel` outbound to the manager's portal port. The bridge client id is a stable `util.StableUuid` (`reverseRrBridgeId`), so both sides agree without shared state. Enabled when both `XraySettings.ReverseRrManagerPort` (clients) and `XraySettings.ReverseRrPortalPort` (bridges) are set. Tunnel uses no flow because reverse carries mux (incompatible with `xtls-rprx-vision`).

## External Links
- [Xray: Proxy Platform](https://github.com/XTLS/Xray-core)
- [Xray Config Examples](https://github.com/XTLS/Xray-examples)

## AI Development Guidance
- Keep it simple, stupid.
- Prefer small, inline improvements; keep the architecture intact.
- The Xray binary is embedded (single pinned version), so target exactly that version. When Xray changes its output or behavior, update to the new format and drop old-version handling — do not keep backward-compatibility branches.
