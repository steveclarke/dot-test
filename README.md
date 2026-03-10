# dot-test

Named `.test` URLs for local dev servers. Stop memorizing port numbers.

```
http://myapp.test    instead of    http://localhost:3000
http://nexus.test    instead of    http://localhost:3013
http://praxis.test   instead of    http://localhost:3002
```

dot-test automatically discovers Rails projects in the current directory, assigns stable port numbers, and runs a local DNS server + HTTP reverse proxy so `*.test` domains route to the right backend.

Zero dependencies. Single binary. Works with however you start your apps.

## Install

```sh
brew install dot-test
```

Or build from source:

```sh
go install github.com/zarpay/dot-test@latest
```

## Quickstart

```sh
cd ~/projects         # or wherever your apps live
dot-test setup        # one-time macOS DNS config (needs sudo)
dot-test sync         # discover projects, assign ports
dot-test up           # start the daemon
```

Open `http://yourapp.test` in your browser.

## How It Works

1. **Discover** — Scans your projects directory for Rails apps (looks for `config/application.rb`)
2. **Assign** — Gives each app a stable port number starting from 3000, persisted across runs
3. **Update** — Writes `PORT=<assigned>` to each app's `.env` file
4. **Route** — Runs a DNS server (`.test` → `127.0.0.1`) and HTTP proxy (hostname → port)

Your apps read the port from `.env` via dotenv-rails or foreman. You start them normally with `bin/dev`. dot-test handles the rest.

## Commands

```
dot-test [sync]    Discover projects, assign ports, update .env files
dot-test list      Show current port mappings
dot-test up        Start DNS + reverse proxy daemon
dot-test down      Stop daemon
dot-test setup     One-time OS configuration (creates /etc/resolver/test)
dot-test clean     Remove all config and stop daemon
dot-test version   Show version
dot-test help      Show help
```

## Port Assignment

Ports are assigned sequentially starting from 3000 and persist in a `.dot-test` file inside your projects directory.

dot-test respects hardcoded ports. Before assigning, it checks:

- `bin/dev` for `PORT=XXXX` or `--port` flags
- `Procfile.dev` for `-p` port assignments
- `ship-manifest.dev.toml` and `Procfile.toml` for bind addresses
- `docker-compose.yml` for port mappings

Hardcoded ports show as `(pinned)` in output. Other projects are assigned ports around them.

## Example

```
$ dot-test sync
33 projects:

  http://empirium.test   -> localhost:3200 (pinned)
  http://fizzy.test      -> localhost:3122 (pinned)
  http://nexus.test      -> localhost:3013
  http://praxis.test     -> localhost:3002
  http://zar-core.test   -> localhost:3030

Updating .env files...
  33 updated

Done. Run 'dot-test up' to start the daemon.
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DOT_TEST_DIR` | current directory | Directory to scan for projects |
| `DOT_TEST_PORT` | `80` | HTTP proxy listen port |
| `DOT_TEST_DNS_PORT` | `15353` | DNS server listen port |

## .env Compatibility

dot-test writes `PORT=<port>` to each project's `.env` file. For apps to pick this up automatically, they need one of:

- `dotenv-rails` gem in their Gemfile
- `foreman` or `overmind` in `bin/dev` with a `Procfile.dev`

Apps that won't read `.env` are flagged with a warning during `sync`. You can always start those manually:

```sh
PORT=3005 rails s
```

## Error Pages

If you visit a `.test` URL and the backend isn't running, you get:

```
dot-test: nexus.test is not running
Start it with: cd ~/projects/nexus && bin/dev
```

## macOS Setup

`dot-test setup` creates `/etc/resolver/test`, which tells macOS to route DNS queries for `*.test` to dot-test's built-in DNS server. This persists across reboots.

## Linux

For systemd-resolved, add to `/etc/systemd/resolved.conf`:

```ini
[Resolve]
DNS=127.0.0.1:15353
Domains=~test
```

Then restart: `sudo systemctl restart systemd-resolved`

## License

MIT
