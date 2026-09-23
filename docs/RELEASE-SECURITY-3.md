# Security 3: backup integrity and explicit proxy trust

Install the final patch tag `v1.2.5-fix2-security.3.1`. The initial `security.3` candidate was superseded during final review; its tag is retained for traceability and its server-image publication was cancelled. The final patch also prevents repeated extraction after a failed restore and preserves empty backup directories.

This update keeps the Classic 1.2.5-fix2 application family. It changes backup handling and how client IP addresses are trusted. It does not migrate upstream 1.2.7 history or recover records missing from an earlier import.

## Backup changes

- Exports include a versioned manifest and file checksums. The complete archive is validated before the download starts.
- Imports require this format, check every archive entry and checksum, and check the SQLite database before queuing a restart. Files are checked again at startup before live data moves.
- Backups exported before Security 3, including Security 1/2 and upstream 1.2.7 backups, are rejected by the automatic importer. Keep those originals for separate recovery/migration. Do not add or edit a manifest to bypass the restriction.
- Restoring replaces accounts, password hashes, MFA secrets, agent tokens, settings and data with the backup's values. Browser sessions are revoked. Use only your own trusted backups: checksums detect corruption, not a malicious administrator who can rewrite both the payload and manifest.
- The previous installation, including its SQLite WAL/SHM files, is retained under `/app/data/.komari-restore/restore-*/previous` on the persistent data volume. The uploaded source ZIP is retained beside it. These private recovery files are excluded from subsequent exports.
- Ordinary file-move errors trigger rollback. Failed or interrupted restores retain `/app/data/.restore-in-progress`; startup stops instead of opening a partial database or repeatedly extracting the same archive during Docker restart loops. Recovery requires an operator to address the error first. This is a staged multi-file replacement, not an atomic filesystem transaction or a guarantee against storage/power failure.
- Custom database locations are rejected by the automatic importer. Extra database files such as an upstream `metrics.db` cause export/import to fail explicitly rather than silently omitting that database. Existing upload and expansion limits remain 512 MiB compressed / 2 GiB expanded / 10,000 entries.

Upgrading a running Classic installation does not itself restore a backup. Keep a stopped-volume copy and the current image before upgrading, then export a fresh backup with the new version. Leave a working upstream installation and its full backup intact until any migration is separately verified.

Recovery files contain credentials and consume disk space. Keep adequate free space for the uploaded ZIP, extracted replacement and previous data. After verifying a restore and saving an independent offline backup, an operator may remove the completed recovery directory. Do not delete an active journal or its recovery directory to force startup.

### If startup stops during restoration

Stop the container/restart loop and take a complete copy of its data volume before intervening. Preserve the journal, pending ZIP, and the entire `.komari-restore` directory. The journal records the transaction directory and old/new top-level names. `previous` may be incomplete if interruption occurred while moving the old files; remaining originals may still be at the data root. A failed rollback can also leave files in both places. Inspect those locations before recovery. Prefer restoring a known complete, stopped-volume backup with its matching image. Never delete SQLite WAL files or blindly copy only `komari.db` from a running installation.

## Reverse proxy setup

Gin no longer trusts every peer's forwarded IP headers. With no additional configuration, client-IP logging and the login IP budget use the actual socket peer. Behind a proxy, users consequently share that peer's IP budget. Account and server-wide login budgets continue to apply.

To use individual visitor IPs, set `KOMARI_TRUSTED_PROXIES` to the exact IP(s) or narrow CIDR(s) of your controlled reverse proxy, separated by commas. For example, add an `environment` entry to your existing Compose service after determining its actual proxy peer:

```yaml
environment:
  KOMARI_TRUSTED_PROXIES: "YOUR_ACTUAL_PROXY_IP"
```

The placeholder must be replaced. Do not assume the proxy's public address is the connection peer seen inside Docker. Host Nginx often appears as a Docker bridge gateway. Do not trust all addresses or an entire shared network. Invalid entries and `/0` networks stop startup.

The proxy must overwrite `X-Forwarded-For` with a validated visitor address; do not copy an arbitrary incoming header. Forwarded scheme headers must also be overwritten correctly for HTTPS cookies and OAuth. If Cloudflare is in front of Nginx, configure Nginx's real-IP module to accept `CF-Connecting-IP` **only from Cloudflare's official proxy ranges**, then pass the resulting address to Komari. Keep the Komari port private. See [Cloudflare's original visitor IP guidance](https://developers.cloudflare.com/support/troubleshooting/restoring-visitor-ips/restoring-original-visitor-ips/).

These source changes do not alter any existing VPS, Nginx, Cloudflare, DNS, account or MFA configuration. Those settings need deployment-specific verification.

## Validation

Regression tests cover rejection of unsupported/corrupt backups, preservation of original data, failures at each replacement step, persistent recovery files, interrupted startup, session revocation, credential/agent-token/history preservation and spoofed proxy headers. Docker verification additionally downloads and restores an actual running container backup, verifies rejection without data changes, checks exclusive queuing, restarts, checks session revocation and re-exports the result.

Validation completed on 2026-09-23:

- Full local Go suite and production frontend build passed. The pre-existing GeoIP integration tests require public-network access; they passed when that access was available.
- [Source Docker regression run](https://github.com/coolman1232004/komari-classic/actions/runs/35702278740): AMD64 and ARM64 passed, including the new live backup/restore checks and the existing authentication, persistence, agent and race checks.
- [Release workflow](https://github.com/coolman1232004/komari-classic/actions/runs/35807403224): all 25 jobs passed.
- [Published-image verification](https://github.com/coolman1232004/komari-classic/actions/runs/35808710093): AMD64 and ARM64 passed anonymous pulls, installation, backup/restore, session revocation and image scanning.
- Linux-target `govulncheck`: zero affected symbols and zero affected imported packages. One module-level advisory describes the unused OpenPGP package.

| Published image, each architecture | Critical/high | Medium | Low | Unknown |
|---|---:|---:|---:|---:|
| Server | 0 | 0 | 0 | 2 |
| Agent | 0 | 0 | 0 | 0 |

The two server entries are GO-2026-5932 for the unimported `golang.org/x/crypto/openpgp` package in the application and bundled cloudflared module metadata. No fixed version is listed. These are reported rather than silently suppressed; the underlying module is still needed for other cryptographic packages. See the [Security 2 analysis](RELEASE-SECURITY-2.md) for the original reachability assessment.

- Release/source commit: `5bb6687f54a5ddc68f3454db8651c3f80b95ae13`.
- Server multi-architecture digest: `sha256:0a6f56e3fa6df3d917209b0a62424dba257c431fe1023bf6fcafe863fe3389ff`.
- Agent multi-architecture digest: `sha256:1565d886afe534a8a9961eab32651d0f63b5b568602b9a58aa11c6199da2f130`.
- `compose.image.yaml` pins the verified server digest. Downloaded scan artifacts were checked against GitHub's recorded SHA-256 values.

Passing tests or vulnerability scans is not a zero-vulnerability guarantee. This update does not claim to be a new exhaustive audit of every deployment, plugin or future dependency version.
