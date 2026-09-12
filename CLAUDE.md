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

**`profile`/`auth`**, **`shipment`**, and now a **minimal `device`** group
(`list`/`get` only — see below). Everything else (org, template, tag,
automation, webhook, user, provisioning, static-token, upload, oauth) is
designed (see below) but not being implemented yet — don't scaffold those
commands until asked.

**Status: implemented** (`cmd/profile.go`, `cmd/auth.go`, `cmd/shipment.go`,
`cmd/device.go`, `internal/api/{client,oauth,organization,devices,uploads,
shipments}.go`, `internal/config/*`, `internal/output/table.go`).
Per-OS config-file permission enforcement (chmod/icacls) was explicitly
deferred — not implemented yet, still worth doing later per the "Storage"
section below.

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

Wraps the Blynk.Air OTA shipment + upload endpoints. **Verified against the
live docs** (shipments.md, uploads.md, devices.md) before implementing — the
real API is narrower than the original design in a few ways, noted inline
below. Decisions on how to reconcile were made with the user on 2026-09-12;
don't re-litigate them without a reason.

```
shipment list        [--org-id]
shipment get          --id int
shipment stop           --id int
shipment delete           --id int [--force]
```

Real API notes:
- **No pagination on shipment list** (`--page`/`--size`/`--all` are global
  flags but don't apply here — the endpoint just returns everything).
- **No pause/resume** — only `PUT /shipment/stop` exists (valid from RUN or
  PAUSE), and there's no way to resume a stopped/paused shipment afterwards.
  So `shipment pause`/`resume`/`cancel` collapsed into one `shipment stop`.
- **No report/export endpoint** — dropped `shipment report` entirely.
  `shipment get -o json` exposes the same `shipmentProgress` counters anyone
  building a CSV would need.
- Template ID/name surface as `productId`/`productName` on the Shipment
  object (not a separate `templateId` field) — Blynk's usual alphanumeric
  template ids (`TMPL0X9F`) do **not** appear here; both `productId` (on
  Shipment) and `templateId` (on Device) are plain `int32`.

### `shipment deploy` — the combined upload+create convenience command

This is the primary command for the common workflow: upload a firmware
binary and create a shipment targeting it, in one step, without needing to
separately call the uploads endpoint first.

```
blynk shipment deploy \
  --file ./firmware-v2.3.1.bin \
  --device-ids 123,456 \
  [--template-id 421]         \
  [--name string]              \
  [--shipment-time ANY|NIGHT|MORNING|AFTERNOON|EVENING] \
  [--compare-field NO_CONDITION|BUILD_DATE_DIFFERS|EARLIER_BUILD_DATE|LATEST_FIRMWARE_VERSION|LATEST_BLYNK_VERSION] \
  [--skip-fw-type-check] [--attempts-limit 3] [--attempt-reset-period 24h] \
  [--wait] [--no-wait] [--wait-timeout 15m] [--verbose] \
  [--dry-run] [-y/--yes]
```

### ⚠ Gotchas found via live testing against a real MQTT-connected device (2026-09-12)

Three `shipment/create` fields the original design didn't know about turned
out to matter a lot in practice — found by comparing a CLI-created shipment
against an otherwise-identical UI-created one for the same file/device
(`Nvidia ORIN`, template `Linux Agent`, firmware = a `docker-compose.yml` —
this org ships container manifests as "firmware" to edge devices, and `.yml`/
`.yaml` are legitimately on the accepted-upload-extensions list):

- **`attemptsLimit` silently defaults server-side to `0`, which appears to
  mean "make zero delivery attempts"** — a shipment left with no attempts
  configured just sits at `started` forever with no `requestSent`, even
  though the device is online, connected via MQTT, and would otherwise have
  been notified immediately (confirmed: the UI defaults this to `3` attempts
  / 24h and the device gets notified right away). **The CLI now defaults
  `--attempts-limit` to `3` and `--attempt-reset-period` to `24h`** to match
  the UI rather than the API's own default — this isn't a safety-relevant
  default so there was no reason to leave the footgun in place.
- **`skipFwTypeCheck` is now auto-applied when the uploaded file has no
  `fwType` metadata** (added 2026-09-12, after the user pushed back on
  needing `--skip-fw-type-check` by hand every time for `Linux Agent`-style
  devices). The check compares against `firmwareInfo.fwType`, which the
  upload endpoint's parser can only populate from genuine embedded-firmware
  metadata — a plain manifest like `docker-compose.yml` will *always* come
  back with `fwType: ""` (confirmed identical between a CLI and UI upload of
  the same file via matching MD5), so enforcing the check in that case can
  never succeed and adds nothing. **This looks like an actual bug in the
  Platform API** (or at least a real design gap) — a safety check with no
  possible non-failing outcome isn't protecting anything; the fix belongs
  upstream, not in every API caller. `cmd/shipment.go`'s deploy command now
  checks `upload.FirmwareInfo.FwType == ""` right after upload and force-sets
  `SkipFwTypeCheck` in that case, printing a one-line notice; `--skip-fw-type-check`
  still exists as a manual override for the (currently untested) case of a
  file that *does* have real `fwType` metadata but you want the check
  bypassed anyway.
- **`compareField` is still left as an explicit opt-in flag**, not
  auto-applied — unlike the fwType case, there's no "this can never succeed"
  argument for `BUILD_DATE_DIFFERS`; it's a legitimate default that avoids
  redundant re-flashing of a device already on the same build. Pass
  `--compare-field NO_CONDITION` explicitly when you want to force a
  redeploy of an unchanged file (e.g. repeated testing).

**`--tag-id`/`--all-devices` targeting was dropped** (decided 2026-09-12):
`POST /shipment/create` only accepts an explicit `deviceIds` array — no
tag or org/template-wide targeting server-side. Resolving a tag or
"all devices" into a device-id list would require devices-list/by-tag API
calls, which are out of scope until the `device` command group is built.
`--device-ids` is the only targeting flag for now; revisit tag/all-devices
once `device` exists.

**`--device-ids` accepts device names as well as numeric ids** (added
2026-09-12, discovered live testing that device console labels aren't the
numeric id the API needs): each comma-separated token is looked up directly
if numeric, otherwise resolved via `GET /search/devices?query=` (added to
`internal/api/devices.go`), requiring exactly one match — an exact
case-insensitive name match wins over multiple substring matches, and
ambiguous/zero matches are a hard error listing candidates.

**`--version` and `--rollout gradual|immediate` were dropped**: neither maps
to a real field. Firmware version comes from server-parsed `firmwareInfo`,
not a client-supplied value. There's no gradual-vs-immediate concept in the
schema — the closest real field is `shipmentTime` (ANY/NIGHT/MORNING/
AFTERNOON/EVENING, a time-of-day schedule, default ANY), exposed as
`--shipment-time`.

**Template ID resolution** (unchanged from original design, now just
single-path since tag/all-devices is gone): looked up automatically per
device via `GET /device` (`templateId` field). Mixed templates across
`--device-ids` is a hard error — a shipment targets one template. If
`--template-id` is also passed, a mismatch against what the device(s)
actually report is a loud error, not a silent override.

**Name resolution** — `--name` is optional, auto-generated when omitted:
- Single device: `<device-name> · <firmware-filename> · <timestamp>`
  → `boiler-3 · fw-2.3.1.bin · 2026-09-12 14:32`
- Multiple `--device-ids`: `template <id> · <firmware-filename> · <timestamp>`
- Timestamp avoids name collisions (shipment titles must be unique) on
  back-to-back re-runs.

**Confirmation prompt** before firing (unless `-y`/`--yes`), showing resolved
device name/count and firmware file.

**`--wait` — poll until the rollout finishes:**
There is **no per-device status endpoint** — only one aggregate
`shipmentProgress` object per shipment, with running counts (int32) across
all its devices: `started`, `requestSent`, `firmwareRequested`,
`firmwareUploaded`, `firmwareUploadedToMobile`, `success`, plus failure
counters `uploadFailure`, `firmwareTypeMismatch`, `downloadLimitReached`,
`rollback`, `firmwareVersionMismatch`. The shipment's own `status` field
(RUN/PAUSE/FINISH/CANCEL) says when it's done.
- **Default ON for exactly one device**, default OFF otherwise — same as
  originally designed.
- Single device: infer a stage label from which counters have gone from 0 to
  ≥1 (started → notified → firmware requested → firmware uploaded → success,
  or one of the failure labels), print a line each time the stage advances.
  There's no literal "downloading" state in the real API — dropped that
  label rather than fabricate it.
- Multi-device: tally line refreshed in place
  (`Rolling out... N/total updated, F failed, P pending`); `--verbose` prints
  the raw counters every tick instead.
- Failure → non-zero exit, terminal line points at `shipment get --id`
  (no report command to point to anymore).
- `--wait-timeout` (default 15m) as originally designed.

**`--dry-run`**: resolves `--device-ids` into real device names/templates
and prints them without uploading or creating anything.

### Example end-to-end run (single device, the common case)

```
$ blynk shipment deploy --file ./fw-2.3.1.bin --device-ids 12345
Resolved device 12345 → template 421 (boiler-3)
About to deploy fw-2.3.1.bin to 1 device(s) (boiler-3, id 12345) as shipment
"boiler-3 · fw-2.3.1.bin · 2026-09-12 14:32". Continue? [y/N] y
✓ Uploaded fw-2.3.1.bin (1.2 MB)
✓ Shipment 8843 created

  boiler-3: notified
  boiler-3: firmware requested
  boiler-3: firmware uploaded to device
  boiler-3: applied ✓

✓ Shipment 8843 complete — firmware applied successfully
```

## Suggested project layout

```
blynk-cli/
  main.go
  cmd/
    root.go            # root command, global flags, requireClient()/resolveOptsFromFlags()
    profile.go          # profile add/list/use/remove/show/rename/switch
    auth.go              # auth login/token/whoami/logout
    shipment.go           # shipment list/get/stop/delete/deploy
    device.go              # device list/get (minimal — online status + firmware version)
    prompt.go             # masked secret prompt, y/N confirm
    picker.go              # profile switch's interactive fzf-style picker
  internal/
    config/
      config.go          # ~/.blynk/config.yaml read/write, Profile/Config types
      resolve.go           # the 5-tier profile resolution order + prefix/substring matching
      token.go              # OAuth2 token cache/refresh bridge to internal/api
                          # (permission enforcement per-OS: not implemented yet, deferred)
    api/
      client.go           # shared HTTP client: base URL from profile, bearer
                          # token injection, error unwrapping
      oauth.go             # client-credentials token fetch + expiry tracking
      organization.go        # GET /organization/profile (whoami + profile-add validation)
      devices.go             # GET /device, /devices (list), /search/devices, /device/online
      uploads.go              # POST /api/upload (multipart firmware upload)
      shipments.go             # typed request/response structs + calls for the Shipments endpoints
    output/
      table.go            # shared table/JSON/YAML renderer used by every command
```

`internal/api` uses typed Go structs per resource (not raw maps) so
`shipments.go`/`devices.go` become the copy-paste template for the full
`devices.go`, `templates.go`, etc. when those command groups are built later.
`devices.go` today is intentionally minimal (id/name/templateId) — expand it
in place rather than creating a second file when the `device` command group
is built.

## Testing

`go test ./...` runs the unit suite (fast, no network/credentials needed):
- `internal/config`: profile CRUD, prefix/substring resolution, the full
  5-tier precedence order (using `t.Chdir`/`t.Setenv`, Go 1.24+), UTF-8 BOM
  stripping for both `config.yaml` and `.blynk-profile`.
- `internal/api`: JSON schema decode tests, several using **real captured
  response bodies from live QA testing** (not synthetic) — e.g. the full
  `Shipment` fixture from the docker-compose.yml/Nvidia ORIN investigation
  (`shipments_test.go`), and the nested `{"error":{"message"}}` body that
  broke error parsing the first time it was hit for real (`client_test.go`).
  When you discover another live schema surprise, prefer adding it here as a
  fixture over just fixing the struct — it's what caught the `Organization`
  `address`/`id` bugs early once written.
- `internal/output`: table alignment, JSON/YAML/table dispatch (100% covered).
- `cmd`: pure logic (device-token parsing, auto-naming, byte formatting,
  `--wait` stage inference) plus a mocked-HTTP-server test of
  `resolveDevices`/`resolveDeviceToken` (numeric id, exact-name match,
  ambiguous names, mixed-template hard error) via `httptest.NewTLSServer`.

`internal/api/integration_test.go` holds a small **opt-in, read-only**
integration suite that hits a real server — skipped by default, runs when
`BLYNK_TEST_SERVER`/`BLYNK_TEST_TOKEN` (a static token) are set:
```
BLYNK_TEST_SERVER=fra.blynk-qa.com BLYNK_TEST_TOKEN=... go test ./internal/api/... -run Integration -v
```
Deliberately limited to read-only calls (`whoami`, `shipment list`) — it
must never create/mutate anything, since it can run against a real org.

## `device` command group (minimal — added 2026-09-12)

Added on request, scoped narrowly to two things: checking whether a device
is online, and seeing its firmware version. Not the full device command
group from the original design sketch (no create/edit/delete/datastream/
tag-assignment/etc.) — ask before expanding this beyond `list`/`get`.

```
device list  [--org-id] [--include-sub-org-devices] [--page] [--size] [--all] [--online] [--reveal]
device get    --id <id-or-name> [--reveal]
```

**`device list --online`** (added 2026-09-12, on request): shows a live
online status per row too. Off by default, since it's a separate call to
`/device/online` per device (not part of the list response) — opting in
means one extra request per row, which would be surprising as a default for
a large `--all` listing. `fetchOnlineStatuses` in `cmd/device.go` bounds
concurrency (8 at a time) rather than firing every request at once or doing
them serially; a single device's request failing renders `?` for just that
row instead of failing the whole listing.

Verified against the live docs before implementing:
- `GET /organization/devices` (list, paginated `{content, totalElements}`,
  same shape as the search endpoint — factored into a shared `devicePage`
  type in `internal/api/devices.go`) **does** include `hardwareInfo` (so
  firmware version is free in `device list`, no per-row extra calls) but
  **not** `lifecycleStatus`/connect metadata (only the single-device
  `GET /device` has that).
- **Device connectivity is a separate endpoint entirely**:
  `GET /organization/device/online?deviceId=` → `{"connected": bool}`. It is
  *not* part of the Device object, and `lifecycleStatus` (present on
  `GET /device`) is a provisioning/lifecycle state, not live connectivity —
  don't conflate the two.
- `hardwareInfo.templateId` is a **string**, the alphanumeric id embedded in
  firmware (e.g. `TMPL0X9F`) — a different value in a different format from
  the numeric `Device.templateId` (int32). Same field name, different
  meaning; see `DeviceHardwareInfo.TemplateID`'s doc comment.

`device get --id` accepts a device name as well as a numeric id (reuses
`resolveDeviceToken` from `cmd/shipment.go` — same search-based resolution
`shipment deploy --device-ids` uses), then always re-fetches via
`GetDevice` for a consistent full schema regardless of which path resolved
it, then calls `IsOnline` separately.

**`-q`/`--quiet` on `device get` prints just `online` or `offline`** — this
was clearly the intended use of that global flag; its doc string already
said "e.g. just `online`/`offline`" before this command existed.

The device's own auth `token` field is a credential — hidden by default in
both table and JSON/YAML output (redacted before rendering, not just
omitted from the table), shown only with `--reveal`, mirroring
`profile show`'s existing convention. **`device list` needs this too, not
just `get`** — the list endpoint returns `token` per device same as the
single-device one, so its JSON output was leaking every listed device's
auth token before this was caught (fixed same day as `--online`, via the
`deviceListItem` wrapper type rather than rendering raw `[]api.Device`).

## Also designed but NOT being built yet

Full command-tree sketch exists for: `org`, `datastream`, `template`
(+ nested `datastream`/`event`/`metafield`), `tag`, `automation`, `webhook`,
`user`, `provision`/`static-token`, `upload`, `oauth`, and the rest of
`device` beyond `list`/`get` (create/edit/delete/datastream/tag-assignment/
etc.). Ask before scaffolding these.

## Environment

- Go 1.26 on Windows (dev machine), targeting cross-compilation for
  Windows/Mac/Linux via `GOOS`/`GOARCH`.
- Module: `github.com/anthony-blynk/blynk-cli`
- VS Code with the official `golang.go` extension;
  `go.useLanguageServer: true`, `editor.formatOnSave: true`,
  `go.lintTool: staticcheck` set in `.vscode/settings.json`.
