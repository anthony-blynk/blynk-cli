# blynk-cli

Command-line client for the [Blynk Platform API](https://docs.blynk.io/en/blynk.cloud/platform-https-api),
for Blynk employees working across many customer orgs/servers.

## Install

Binaries for Windows, macOS, and Linux (amd64 + arm64) are published on the
[latest release](https://github.com/anthony-blynk/blynk-cli/releases/latest).
The commands below install `v0.1.0` specifically — check the releases page
for a newer version and swap it into the URL if one exists.

### Linux / Raspberry Pi

Check your architecture first:

```bash
uname -m
```

`aarch64` → arm64 (e.g. a 64-bit Raspberry Pi OS on a Pi 4/5); `x86_64` → amd64.

```bash
curl -L https://github.com/anthony-blynk/blynk-cli/releases/download/v0.1.0/blynk-cli_0.1.0_linux_arm64.zip -o /tmp/blynk.zip \
  && unzip -o /tmp/blynk.zip -d /tmp/blynk-cli \
  && sudo install -m 755 /tmp/blynk-cli/blynk /usr/local/bin/blynk \
  && blynk --version
```

(swap `linux_arm64` for `linux_amd64` in the URL on an x86_64 machine)

### macOS

Check your architecture first (`arm64` = Apple Silicon, `x86_64` = Intel):

```bash
uname -m
```

```bash
curl -L https://github.com/anthony-blynk/blynk-cli/releases/download/v0.1.0/blynk-cli_0.1.0_darwin_arm64.zip -o /tmp/blynk.zip \
  && unzip -o /tmp/blynk.zip -d /tmp/blynk-cli \
  && sudo install -m 755 /tmp/blynk-cli/blynk /usr/local/bin/blynk \
  && blynk --version
```

(swap `darwin_arm64` for `darwin_amd64` on an Intel Mac)

### Windows (PowerShell)

```powershell
Invoke-WebRequest -Uri "https://github.com/anthony-blynk/blynk-cli/releases/download/v0.1.0/blynk-cli_0.1.0_windows_amd64.zip" -OutFile "$env:TEMP\blynk.zip"
Expand-Archive -Path "$env:TEMP\blynk.zip" -DestinationPath "$env:LOCALAPPDATA\blynk-cli" -Force
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*blynk-cli*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$env:LOCALAPPDATA\blynk-cli", "User")
}
& "$env:LOCALAPPDATA\blynk-cli\blynk.exe" --version
```

Open a new terminal afterward so the updated `PATH` takes effect. (An
ARM64 build is also published, for ARM-based Windows devices.)

## Usage

```bash
blynk profile add my-org --server my-org.blynk.cloud --client-id <id> --use
blynk auth whoami
blynk device list
blynk device get <id-or-name>
blynk shipment deploy --file firmware.bin --devices 12345
```

Have a static access token instead of OAuth2 client credentials? Use
`--token <value>` in place of `--client-id`/`--client-secret`.

Run `blynk <command> --help` (or `blynk <command> <subcommand> --help`) for
full details on any command.

## License

MIT — see [LICENSE](LICENSE).
