# AGENTS.md

## Project summary
P-Manager is a Go service that manages users and remote P-Nodes, generates Xray configs, and serves a static admin UI.
It stores state in JSON files under `storage/` and syncs configs and stats between the local Xray instance and P-Nodes.

## Runtime Flow
- `main.go` → `cmd/root.go` (Cobra) → `cmd/serve.go`.
- `internal/app.New()`: Builds config, logger, database, xray, http server, http client, coordinator, and other modules.
- `App.Run()`: Initializes and runs long-running modules like database, xray, coordinator, and http server, etc.
- Signals handled in `internal/app/app.go` to stop gracefully.

## Tech stack
- Main: Go 1.25.x, Cobra CLI, Echo HTTP server
- UI: HTML + jQuery + Tabulator + Bootstrap

## Packages
- `cmd`: Cobra commands
- `configs`: Configuration files
- `internal/app`: App lifecycle orchestration
- `internal/composer`: Xray configuration generator for local and remote Xray instances
- `internal/config`: Configuration loader, constants, and paths
- `internal/coordinator`: Periodic sync between current state, local Xray process, and P-Nodes
- `internal/data`: database schema, models and default values
- `internal/http`: Echo HTTP server, routes, and API handlers
- `pkg/util`: Generic utils
- `scripts`: Scripts for project and server setup
- `storage`: Application data storage directory
- `third_party`: Third-party binaries and libraries
- `web`: Static admin web UI files

## Database

- Database is simple file-backed JSON store
- Database driver is `pkg/database`
- Database schema is defined in `internal/data/data.go`
- Database file path is `storage/database/data.json` (`DatabaseFilePath` in `internal/config/config.go`)
- Database backup path is `storage/database/backup-%s.json` (`DatabaseBackupPath` in `internal/config/config.go`)

## HTTP APIs
- All handlers are located in `internal/http/handlers/api`
- Requests and responses are in JSON format
- Authentication is token-based, token is stored in database (`admin_password` in `internal/data/main_settings.go`)
- Header `Authorization: Bearer <token>` is checked for authenticated routes

## Major Dependencies
- `github.com/xtls/xray-core`: Interaction with running Xray process
- `github.com/labstack/echo`: HTTP server and router
- `github.com/spf13/cobra`: CLI framework
- `github.com/go-playground/validator`: Validating config models
- `github.com/cockroachdb/errors`: Errors and stacktrace
- `go.uber.org/zap`: Logging

## Key Behavior Notes / Gotchas
- Admin panel username is `admin` and password is defined in `internal/data/main_settings.go` as `AdminPassword`
- Default admin password is `password` and could be changed after first login
- `make update` is destructive (`git reset --hard` + `git clean -fd`)
- The built `p-manager` binary is tracked and should be committed when updated
- It uses some packages from P-Node repository ("github.com/miladrahimi/p-node")
- On local environment P-Node could be replaced by `../p-node` in `go.mod` to use local version of P-Node
- Use Java-style camelCase for namings (`UserId` instead of `userID`, `clientId` is `clientID`, etc.)

## Xray
Xray is a proxy platform which can be used to run proxy servers with different protocols like Shadowsocks, VMess, VLess,
Socks, etc. It supports chain of proxies. The high level flow is:

```
[ Client ] -> [ Xray on Server 1 ] -> [ Xray on Server 2 ] -> Internet
```

Each xray node receives traffic on its inbound port and forwards it to the next node in the chain.
Based on the routing rules and balancers it forwards traffic to the appropriate outbound.

Xray provides a reverse proxy feature that allows a connection to be initiated from the next node back to the previous
node. This is particularly useful when the previous node is behind a firewall (such as the GFW) that restricts outbound
connections to the network where the next node resides.

## SSH Proxy
SSH Proxy is the same socks proxy provided by SSH tool. The provided SOCKS proxy will be used as outbound for Xray.

## External Links
- [Xray: Proxy Platform](https://github.com/XTLS/Xray-core)
- [Xray Config Examples](https://github.com/XTLS/Xray-examples)

## AI Development Guidance
- Keep it simple stupid.
- Prefer small, inline improvements; keep architecture intact.
