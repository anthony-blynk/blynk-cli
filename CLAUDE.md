# blynk-cli — Project Brief

A cross-platform (Windows/Mac/Linux) command-line client for the
[Blynk Platform API](https://docs.blynk.io/en/blynk.cloud/platform-https-api),
built for Blynk employees who work across many customer orgs/servers.

Language: **Go** (Cobra for commands, Viper optional for config).
Module: `github.com/anthony-blynk/blynk-cli`

This file is read automatically by Claude Code at the start of a session in
this repo — keep it updated as decisions change so context isn't lost between
sessions.

## Current scope (build this first)

Only two command groups for now: **`profile`/`auth`** and **`shipment`**.
Everything else (device, org, template, tag, automation, webhook, user,
provisioning, static-token, upload, oauth) is designed (see below) but not
being implemented yet — don't scaffold those commands until asked.

## Why Go

Single static binary per OS via cross-compilation
(`GOOS=windows/darwin/linux GOARCH=amd64 go build`), no runtime required on
target machines. Cobra (used by kubectl/gh/docker) gives subcommands, flags,
help text, and shell completion for free — good fit for wrapping a large
REST API surface.

## Global conventions

```
blynk [global flags] <resource> <action> [resource flags]
```

Persistent/global flags on every command:
```
--server string        Blynk server domain (only needed if not using a profile)
--token string          Bearer access token (or $BLYNK_TOKEN)
--client-id string      OAuth2 client id (or $BLYNK_CLIENT_ID)
--client-secret string  OAuth2 client secret (or $BLYNK_CLIENT_SECRET)
--profile string        Named profile to use for this call only
--org-id int            Override org ID for this call
-o, --output string     table|json|yaml (default "table")
-q, --quiet             Minimal output (e.g. just "online"/"offline")
--page int               0-indexed page (default 0)
--size int               Page size, max 1000 (default 50)
--all                    Auto-paginate, fetch every page
-y, --yes                Skip confirmation prompts
```

All resource identifiers are **flags**, never positional args (`--id`,
`--device-id`, `--org-id`, etc.) — this mirrors the actual Platform API, which
takes every identifier as a query parameter rather than a path segment. This
also disambiguates commands needing two IDs (e.g. future `tag assign
--tag-id --device-id`).

## Auth & profiles (build first — everything else depends on it)

### Problem being solved
Blynk employees swap between many customer servers/orgs all day. OAuth2
client-credentials plumbing must not be re-entered per command, but a single
global "current profile" also causes cross-terminal interference when working
two customers at once.

### Storage (v1 — plaintext, no keychain yet)

One file, not split:
```
Windows: %USERPROFILE%\.blynk\config.yaml
Mac/Linux: ~/.blynk/config.yaml
```
```yaml
current_profile: acme-prod
profiles:
  acme-prod:
    server: acme.blynk.cloud
    client_id: abc123
    client_secret: xxxxxxxxxxxx     # plaintext for now, see "Future" below
  acme-qa:
    server: acme-qa.blynk-qa.com
    token: staticTokenValue123      # static-token auth, no client secret
```

- Restrict the file's permissions the moment it's created — `chmod 0600` on
  Mac/Linux; on **Windows this needs ACLs (`icacls`) instead, since chmod is
  meaningless there** — implement per-OS via `runtime.GOOS` branching.
- On every command startup, check permissions and **warn (don't fail)** if
  they've drifted (e.g. someone widened them, or the dir is on a shared/
  network home directory).
- Non-secret metadata (server, auth type, current_profile) is fine as
  plaintext regardless — it's pointers, not the secret.
- **Future, not now:** move secrets into the OS keychain (macOS Keychain /
  Windows Credential Manager / Linux Secret Service) via a keyring library,
  with a per-profile `secret_store: keychain` field and a
  `blynk profile migrate-to-keychain` command for existing plaintext
  profiles. Fall back to the plaintext file (gated behind
  `--insecure-file-store`) when no OS backend is available (headless Linux).
  Don't build this yet — just don't design anything that blocks adding it
  later.

### Commands

```
profile add <name> --server host --client-id id [--client-secret secret] [--use]
profile add <name> --server host --token static-token-value [--use]
profile list
profile show <name>              # never prints the secret; --reveal to show it
profile use <name>
profile remove <name>
profile rename <old> <new>
profile switch                   # interactive fuzzy picker (fzf-style) over all profile names

auth login   --client-id --client-secret [--server]   # equivalent to profile add, for scripting
auth token                                             # print current cached token
auth whoami                                            # resolve token -> org/user info
auth logout
```

- `profile add` **prompts for the secret** (masked input) if
  `--client-secret` is omitted, rather than requiring it as a flag — keeps it
  out of shell history/`ps aux`. Flag form still works for scripting.
- `profile add` **validates immediately** — attempts a real token fetch (or a
  lightweight profile-info call for static tokens) before saving, so a
  mistyped secret fails at add-time with a clear error, not on first real use.
- `--use` on `add` sets it as `current_profile` in the same step.
- Profile name matching (in `--profile`, `$BLYNK_PROFILE`, `profile use`)
  should accept an **unambiguous prefix/substring** (e.g. `--profile acme`
  matches `acme-prod` if it's the only match); ambiguous matches print
  candidates and exit non-zero rather than guessing.
- Shell completion (Cobra generates this) should list/filter profile names —
  this is the biggest quality-of-life win for a large profile list.

### Resolution order (highest to lowest precedence)

1. `--profile <name>` flag (one-off, doesn't change `current_profile`)
2. Fully-specified `--server`/`--token`/`--client-id`/`--client-secret` flags
   (bypasses profiles entirely — good for CI)
3. `$BLYNK_PROFILE` env var (**the main mechanism for per-terminal-tab
   context** — e.g. `export BLYNK_PROFILE=acme-prod` in one tab,
   `streetleaf-prod` in another, no interference)
4. A `.blynk-profile` file in cwd or a parent dir (optional, directory-scoped
   convenience, like `.nvmrc`/direnv — lower precedence than `$BLYNK_PROFILE`
   so it's a default, never a surprise override)
5. `current_profile` from config.yaml

### Token lifecycle

For `client_credentials` profiles: fetch token on first use, cache
(access token + expiry) alongside the profile, refresh transparently shortly
before expiry. For `token` (static) profiles: send as-is, nothing to refresh.

## `shipment` command group (build second)

Wraps the Blynk.Air OTA shipment + upload endpoints.

```
shipment list        [--org-id] --page --size --all
shipment get          --id int
shipment pause         --id int
shipment resume         --id int
shipment cancel          --id int
shipment delete           --id int [--force]
shipment report            --id int --output csv
```

### `shipment deploy` — the combined upload+create convenience command

This is the primary command for the common workflow: upload a firmware
binary and create a shipment targeting it, in one step, without needing to
separately call the uploads endpoint first.

```
blynk shipment deploy \
  --file ./firmware-v2.3.1.bin \
  (--device-ids 123,456 | --tag-id 42 | --all-devices) \
  [--template-id TMPL0X9F]     \
  [--name string]              \
  [--version string]            \
  [--rollout gradual|immediate]  \
  [--wait] [--no-wait] [--wait-timeout 15m] [--verbose] \
  [-y/--yes]
```

**Device targeting is required and mutually exclusive** — exactly one of
`--device-ids`, `--tag-id`, `--all-devices`. No silent default; refuse to run
without one specified, since accidentally shipping to every device is the
single most dangerous mistake this whole CLI can make.

**Template ID resolution:**
- `--device-ids` → template is **looked up automatically** per device (a
  `device get` call returns `templateId`). If the given device IDs span
  *different* templates, that's a hard error (a shipment is firmware for one
  template — mixed templates means two shipments, not one).
- If `--template-id` is *also* passed alongside `--device-ids`, it's checked
  against what the device(s) actually report; a mismatch is a loud error, not
  a silent override.
- `--tag-id` / `--all-devices` → `--template-id` **stays required**, since
  these can span many devices/templates and there's no single device to
  infer it from.

**Name resolution** — `--name` is optional, auto-generated when omitted so
the common single-device test case needs zero naming thought:
- Single device: `<device-name> · <firmware-filename> · <timestamp>`
  → `boiler-3 · fw-2.3.1.bin · 2026-09-12 14:32`
- Tag/all-devices: `<template-name> · <firmware-filename> · <timestamp>`
- Timestamp included specifically to avoid name collisions (shipment names
  must be unique) if the same test is re-run back to back.

**Confirmation prompt** before firing (unless `-y`/`--yes`), showing resolved
device name/count and firmware file — this is the highest-blast-radius
action in the tool, so no silent execution:
```
About to deploy fw-2.3.1.bin to 1 device (boiler-3, id 12345) as shipment
"boiler-3 · fw-2.3.1.bin · 2026-09-12 14:32". Continue? [y/N]
```

**`--wait` — poll until the OTA rollout finishes:**
The shipment API tracks per-device progress through states (initiated →
notified → downloading → applied, plus failure/mismatch states) and the
shipment reaches a terminal state once every device resolves.
- **Default ON when targeting exactly one device** (the common test-on-one-
  board case) — override with `--no-wait`.
- **Default OFF for `--tag-id`/`--all-devices`** multi-device shipments —
  opt in explicitly with `--wait`, since a large rollout may take a while.
- Single device, `--wait`: stream one line per state transition, e.g.
  ```
    boiler-3: initiated
    boiler-3: notified
    boiler-3: downloading
    boiler-3: applied ✓

  ✓ Shipment 8843 complete — firmware applied successfully
  ```
- Multi-device, `--wait`: aggregate tally by default
  (`Rolling out... 187/214 updated, 2 failed, 25 pending`), full per-device
  breakdown with `--verbose`.
- Failure → non-zero exit code, terminal line shows the failure reason,
  points at `shipment report --id` for details.
- `--wait-timeout` (default ~15m) so one stuck device doesn't hang the CLI
  forever — print whatever state was reached and exit non-zero on timeout.

**`--dry-run`** (not yet fully speced) — should resolve the device-targeting
flag into an actual device count/list and print it without uploading or
creating anything, for double-checking a tag/device-id list before commit.

### Example end-to-end run (single device, the common case)

```
$ blynk shipment deploy --file ./fw-2.3.1.bin --device-ids 12345
✓ Resolved device 12345 → template TMPL0X9F (Boiler Controller v2)
✓ Uploaded fw-2.3.1.bin (1.2 MB)
About to deploy fw-2.3.1.bin to 1 device (boiler-3, id 12345) as shipment
"boiler-3 · fw-2.3.1.bin · 2026-09-12 14:32". Continue? [y/N] y
✓ Shipment 8843 created

  boiler-3: initiated
  boiler-3: notified
  boiler-3: downloading
  boiler-3: applied ✓

✓ Shipment 8843 complete — firmware applied successfully
```

### ⚠ Open gap — verify against real API docs before implementing

The exact JSON request/response schema for the **Shipments** and **Uploads**
Platform API endpoints was **not** confirmed against the live OpenAPI spec
during design (unlike `devices` and `organizations`, which were fetched and
verified). Before writing `internal/api/shipments.go`, fetch and check:
- https://docs.blynk.io/en/blynk.cloud/platform-https-api/shipments.md
- https://docs.blynk.io/en/blynk.cloud/platform-https-api/uploads.md

against the actual field names, upload flow (does upload return a file ID/
URL that shipment create references? multipart vs. pre-signed URL?), and
device-progress-status enum values, since those were inferred from adjacent
docs (device shipment statuses, changelog mentions of gradual rollout) rather
than the endpoint spec itself.

## Suggested project layout

```
blynk-cli/
  main.go
  cmd/
    root.go            # root command, global flags
    profile.go          # profile add/list/use/remove/show/rename/switch
    auth.go              # auth login/token/whoami/logout
    shipment.go           # shipment list/get/pause/resume/cancel/delete/report/deploy
  internal/
    config/
      config.go          # ~/.blynk/config.yaml read/write, profile struct,
                          # permission enforcement (per-OS)
    api/
      client.go           # shared HTTP client: base URL from profile, bearer
                          # token injection, error unwrapping
      oauth.go             # client-credentials token fetch + expiry tracking
      shipments.go          # typed request/response structs + calls for
                          # Shipments + Uploads endpoints
    output/
      table.go            # shared table/JSON/YAML renderer used by every command
```

`internal/api` uses typed Go structs per resource (not raw maps) so
`shipments.go` becomes the copy-paste template for `devices.go`,
`templates.go`, etc. when those are built later.

## Also designed but NOT being built yet

Full command-tree sketch exists for: `org`, `device`, `datastream`,
`template` (+ nested `datastream`/`event`/`metafield`), `tag`, `automation`,
`webhook`, `user`, `provision`/`static-token`, `upload`, `oauth`. Ask before
scaffolding these — scope for now is `profile`/`auth` + `shipment` only.

## Environment

- Go 1.26 on Windows (dev machine), targeting cross-compilation for
  Windows/Mac/Linux via `GOOS`/`GOARCH`.
- Module: `github.com/anthony-blynk/blynk-cli`
- VS Code with the official `golang.go` extension;
  `go.useLanguageServer: true`, `editor.formatOnSave: true`,
  `go.lintTool: staticcheck` set in `.vscode/settings.json`.
