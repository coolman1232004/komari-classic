# Komari Classic security maintenance

This branch preserves the 1.2.5-fix2 feature set and adds targeted security and Docker changes. It is not a guarantee that all vulnerabilities have been found. Review date: 2026-09-16.

This document preserves the baseline comparison and early review history. Findings below describe issues addressed during development, not a list of unresolved issues in the current release. See [Security 2](RELEASE-SECURITY-2.md) for the final published-image results dated 2026-09-19 and remaining findings.

## Preserved baseline

The untouched baseline is commit `ea2d85b52ccbb211c144235b9cb51b67d4380cd8`, saved in `coolman1232004/komari-classic` before development.

| Component | Upstream tag | Upstream commit | Identical files | Changed upstream files | Omitted upstream files |
|---|---|---|---:|---:|---:|
| Server | 1.2.5-fix2 | 2f70b440405c4ea70ff3bcbd87361bbb39dc6f60 | 228 | 10 | 7 |
| Frontend | 1.2.5-fix2 | 3e76c4c26c55fc3eaa2a8210322e1b2cacb57e52 | 450 | 9 | 6 |
| Agent | 1.2.60 | 8cd92149a845c12917e42acb1a296c836822758d | 69 | 4 | 6 |

Counts compare SHA-256 of upstream file contents against their mapped paths in Classic. New Classic-only files are not included in these counts. The comparison is against **1.2.5-fix2**, not the earlier plain 1.2.5 release.

In the preserved baseline, all upstream server Go files and dependency locks match. Baseline server differences are documentation, installation and build workflows. The five changed frontend source files change repository/readme links, release queries, installer URLs and agent Docker image names. The baseline agent updater changes its repository and filters release assets by the agent prefix. Thus the baseline preserves the monitoring implementation but changes installation/update behavior; it is not byte-identical upstream. These comparison results predate this fork's subsequent security changes.

## Findings addressed in this branch

* Legacy POST reports trusted a payload UUID over the authenticated agent identity. A compromised agent could submit reports for another node. Reject mismatched UUIDs; retain administrator reporting behavior.
* HTTP access logs included the full query string, including agent tokens, OAuth codes and terminal OTPs. Log paths without query strings. Existing logs are not changed; rotate any credentials that were exposed through shared old logs.
* Request bodies, decompressed agent reports and WebSocket messages had unbounded readers. Apply an 8 MiB message limit and a 512 MiB streaming limit for authenticated backup/theme upload routes. Anonymous identity parsing no longer buffers unrelated uploads. Oversized payloads are rejected; unusually large legitimate payloads may need adjustment.
* Add a 10-second HTTP header deadline and a 60-second idle connection deadline. No global response deadline is imposed on streaming endpoints.
* Stable container agents could still enter binary self-update. All container updates now happen through Docker image replacement.
* Agent images lacked an explicit CA certificate package. Install CA certificates and timezone data.
* Server Docker builds downloaded unpinned cloudflared binaries. Pin 2026.9.1 source by commit and archive checksum, and rebuild with patched, locked dependencies (see the extended audit).

## Existing protections checked

Login rejects absent/invalid enabled 2FA, and session cookies use HttpOnly, SameSite=Lax and Secure when the request scheme is HTTPS. API origin checks default on; WebSocket upgrades use origin validation by default. These protections correspond to earlier upstream advisories:

* https://github.com/advisories/GHSA-jhmr-57cj-q6g9
* https://github.com/advisories/GHSA-q355-h244-969h
* https://github.com/komari-monitor/komari/security/advisories/GHSA-hxjg-93wc-h8p8 (upstream lists 1.2.2 as patched)

Do not disable origin checks or allow arbitrary origins for browser sessions. Use HTTPS and configure your reverse proxy to overwrite forwarded scheme headers. This review does not establish that Nezha vulnerabilities apply to Komari; they are separate implementations.

## Remaining considerations

The follow-up adds Argon2id password storage (19 MiB, two iterations, one lane and an independent 16-byte random salt), following the [OWASP password storage guidance](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html). Legacy fixed-salt SHA-256 hashes migrate on successful password verification using a conditional database update, so a concurrent reset is not overwritten. New accounts, resets and password changes use Argon2id. Existing accounts must authenticate or reset their password to migrate. Back up the database before upgrading: older binaries cannot verify the new hashes. At most four password verifications run simultaneously.

Password login is limited to 10 attempts per account per five minutes, 20 per socket peer per minute and 60 globally per minute. Password and 2FA failures both consume this budget; successful attempts do not reset it. HTTP 429 includes Retry-After. In-memory limits reset on process restart and are per server instance. Socket peers, not user-supplied proxy headers, identify the peer; users behind a reverse proxy share its peer allowance. The bounded limiter fails closed at capacity. This can temporarily restrict legitimate logins during attack, so keep access to the host/CLI recovery path. Login bodies are limited to 16 KiB; usernames/passwords to 256/4096 bytes.

Protect database backups and use a strong unique administrator password plus 2FA. An extended RPC/session, archive, outbound download and compatibility-service review is documented in [SECURITY-AUDIT-2026-09.md](SECURITY-AUDIT-2026-09.md); its limitations still apply. Remote command execution and terminal access are intentional administrator features and make administrator credential protection particularly important.

Docker base-image tags and operating-system package repositories still receive updates. The cloudflared source archive and application dependency lockfiles are pinned, but this is not a claim of byte-for-byte reproducible builds. Save a tested final image by digest for deployment and rebuild deliberately for security updates.

## Validation

The initial npm audit reported 12 advisories (9 high, 2 moderate, 1 low). Resolving fixes within existing package ranges reduced the audit to zero. Go security updates include gRPC 1.83.2, go-jose 4.1.4, x/crypto 0.56.0, x/net 0.58.0 (server), x/image 0.45.0 and the agent's xz 0.5.15, plus their required transitive updates. The Go minimum is now 1.26; source Docker builds use 1.26.8.

The follow-up removes the legacy binary self-updater and its go-github-selfupdate/OpenPGP dependencies entirely. All Classic agents now use explicit installation/image replacement, including non-container builds; automatic release discovery and executable replacement are intentionally retired for the pinned-version policy. Linux agent govulncheck now reports no vulnerabilities. This scan result is limited to the current database and scanned target, not a guarantee against unknown issues.

Regression tests cover cross-node reporting rejection, expanded gzip size limits, fixed/chunked HTTP request limits and omission of secrets from logs. GitHub Actions builds source-based server and agent images, runs their tests, checks the embedded UI and login, rejects unauthenticated and cross-origin admin requests, restarts the server and verifies persistence. See the workflow result for the exact tested commit; a workflow definition alone is not a passing test.
