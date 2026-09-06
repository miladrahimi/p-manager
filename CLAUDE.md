# CLAUDE.md

## Project summary
P-Manager is a Go service that manages users (accounts) and remote P-Nodes, generates Xray configs, and serves a static admin UI.
It stores state in JSON files under `storage/` and syncs configs and stats between the local Xray instance and P-Nodes.

## Runtime Flow
- `main.go` → `cmd/root.go` (Cobra) → `cmd/serve.go` (`serve` command; `start` is a deprecated alias).
- `internal/app.New()`: Builds config, logger, database, http client, xray, ssh client/pool, composer, coordinator, and http server.
- `App.Start()`: Initializes the database, runs the coordinator, and starts the http server.
- `App.Wait()` blocks on the app context; `App.Close()` shuts everything down (http server, xray, ssh pool, saves database, closes logger).
- Signals (`SIGINT`, `SIGTERM`) are handled in `internal/app/app.go` to cancel the context and stop gracefully.

## Tech stack
- Main: Go 1.25.x, Cobra CLI, Echo v4 HTTP server
- UI: static HTML + Alpine.js (behavior) + Tailwind CSS v4 (styling), no build toolchain beyond the Tailwind CLI
- Reuses several packages from the P-Node repo (`github.com/miladrahimi/p-node`): `pkg/database`, `pkg/logger`, `pkg/xray`, `pkg/http/client`.

## Packages
- `cmd`: Cobra commands (`serve`)
- `configs`: Configuration files (`main.defaults.json`, optional `main.json`, `main.example.json`)
- `internal/app`: App lifecycle orchestration
- `internal/composer`: Xray configuration generator for local and remote Xray instances (subpackages `vless`, `socks`; also `link_composer.go` for share links)
- `internal/config`: Configuration loader, constants (`config.go`), and filesystem paths (`paths.go`)
- `internal/coordinator`: Periodic sync between current state, local Xray process, and P-Nodes (`config_syncer`, `stats_syncer`, `ssh_syncer`, `state`)
- `internal/data`: Database schema, models, and default values
- `internal/http/server`: Echo server setup and routing
- `internal/http/handlers`: HTTP handlers (`handlers/api` for JSON APIs, plus the static `account` page handler)
- `internal/worker`: Generic periodic worker (ticker-based background loop)
- `pkg/ssh`: SSH client, connection/proxy config, process management, and connection pool
- `pkg/util`: Generic utils
- `scripts`: Scripts for project and server setup
- `storage`: Application data storage directory (database, logs, generated xray config)
- `third_party`: Third-party binaries and libraries (Xray binaries)
- `web`: Static admin web UI files. Alpine.js is vendored under `assets/third_party`; `assets/js/app.js` holds the shared fetch-based API client and helpers. Styling is Tailwind: edit `assets/css/app.src.css`, then run `make web` to regenerate the committed `assets/css/app.css`.

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
  - `/api/node/*` — node-facing APIs (`GET /api/node/:id/config`); guarded by `Server.authorizeNode` against that node's own `PullToken` (`Node.PullToken`, generated at creation). A node token works only for its own node and never for admin APIs, so the pull command carries no admin credentials.
- Authentication is `Authorization: Bearer <token>`.

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
- `make web` — rebuild the admin UI stylesheet (`web/assets/css/app.css`) via the Tailwind standalone CLI (downloaded to `bin/` on first use); commit the regenerated CSS
- `make local-setup` / `make setup` — local / server setup scripts
- `make clean` / `make fresh` — remove logs / wipe storage state (`app`, `database`, `logs`)
- `make update` — **destructive**: `git fetch` + `git reset --hard` + `git clean -fd` + `git pull` + setup

## Key Behavior Notes / Gotchas
- Admin panel username is `admin`; the password is the `AdminPassword` setting (`internal/data/main_settings.go`).
- Default admin password is `password` (`defaultAdminPassword`); change it after first login. Validation requires 8–32 chars.
- `make update` is destructive (`git reset --hard` + `git clean -fd`).
- The built `p-manager` binary is tracked and should be committed when updated.
- `go.mod` uses `replace github.com/miladrahimi/p-node => ../p-node` to develop against a local P-Node checkout.
- Use Java-style camelCase for namings (`UserId` instead of `userID`, `clientId` instead of `clientID`, etc.).
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

## Supported Methods
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

### Relay RR2SSH
P-Manager works as a relay server.
It starts SSH connections to P-Nodes and provides SOCKS proxies.
It also accepts VLESS Reality Raw requests from clients and forwards them to the SOCKS proxies.
```
[ Client ] -(VLESS Reality Raw)-> [ P-Manager ] -(SOCKS)-> [ SSH to P-Node ] -> Internet
```

### Reverse RR
P-Manager is the reverse portal; each P-Node is a bridge that dials out to it, so P-Nodes work without a public/reachable inbound (only the P-Manager host must be reachable, which nodes learn from the pushed/pulled config). Accounts hit the same manager port and the portal load-balances them across all connected nodes (nodes share the `reverse-rr` domain).
```
[ Client ] -(VLESS Reality Raw)-> [ P-Manager ] ↪ [ P-Node dials back ] -> Internet
```
Config (`internal/composer/config_composer.go`): manager adds a `reverse-rr` inbound (accounts with vision flow + one no-flow bridge client per node) routed to the `reverse-rr-portal`; each node adds a `reverse-rr-bridge` + a no-flow `reverse-rr-tunnel` outbound to the manager. The bridge client id is a stable `util.StableUuid` (`reverseRrBridgeId`), so both sides agree without shared state. Enabled by `XraySettings.ReverseRrManagerPort`. Tunnel uses no flow because reverse carries mux (incompatible with `xtls-rprx-vision`).

## External Links
- [Xray: Proxy Platform](https://github.com/XTLS/Xray-core)
- [Xray Config Examples](https://github.com/XTLS/Xray-examples)

## AI Development Guidance
- Keep it simple, stupid.
- Prefer small, inline improvements; keep the architecture intact.
- The Xray binary is embedded (single pinned version), so target exactly that version. When Xray changes its output or behavior, update to the new format and drop old-version handling — do not keep backward-compatibility branches.
