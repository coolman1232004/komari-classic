# Docker installation from your fork

For a prebuilt image, follow the [repository README](../README.md) and use `compose.image.yaml`. It pins the published multi-architecture digest, uses a separate volume and binds to localhost port 25775. The steps below remain available for source builds.

Fetch the prebuilt-image Compose file from `main`, as linked in the README. The immutable Security 3.1 source tag was created before image publication and still contains the previous Security 2 prebuilt-image references. The verified Security 3.1 digests were recorded on `main` after publication. Existing deployments must preserve their configuration and validate an upgrade on a data copy before changing the image.

**Do not directly downgrade a 1.2.7 backup into this version.** See [release and compatibility notes](RELEASE-SECURITY-3.md).

The unchanged baseline remains at tag `v1.2.5-fix2`; the hardened version has been merged into `main`. For a source build:

```sh
git clone --branch v1.2.5-fix2-security.3.1 https://github.com/coolman1232004/komari-classic.git
cd komari-classic
docker compose up -d --build
docker compose logs komari
```

The server binds to `127.0.0.1:25774` by default. Put an HTTPS reverse proxy in front of it, or access it through an SSH tunnel. To expose the port directly, set `KOMARI_BIND_ADDRESS=0.0.0.0` in `.env` before starting. The proxy must overwrite forwarded scheme headers and forward WebSocket upgrades. Keep API/WebSocket origin checking enabled.

Initial administrator credentials appear in startup logs. Log in, choose a unique password and enable 2FA. Data stays in `./data`. Do not mount the Docker socket or use privileged mode for the server. The Compose configuration prevents privilege escalation and rotates logs. Default Docker filesystem capabilities are retained for compatibility with existing bind-mounted data directories.

This builds the UI and server inside Docker; Go and Node are not required on the host. The existing release Dockerfile remains available for prebuilt release binaries. Prebuilt server and agent images are published under the owner coolman1232004 with version v1.2.5-fix2-security.3.1. The source build remains available independently of GHCR.

## Agent image

Both agent Dockerfiles default to `CMD ["--help"]`. Environment variables alone do not override this command: the container displays help and exits, potentially looping under a restart policy. Supply runtime arguments explicitly. For monitoring without remote terminal/command execution, use `--disable-web-ssh`.

For the published image, create a private `agent.env` file with your endpoint and node token:

```dotenv
AGENT_ENDPOINT=https://YOUR-KOMARI-HOST
AGENT_TOKEN=YOUR-NODE-TOKEN
AGENT_DISABLE_WEB_SSH=true
```

Keep the file outside version control, restrict it to the operator (`chmod 600 agent.env` on Linux), and replace the placeholders before starting:

```sh
docker run -d --name komari-agent --restart unless-stopped \
  --security-opt no-new-privileges:true \
  --log-opt max-size=10m --log-opt max-file=3 \
  --env-file ./agent.env \
  ghcr.io/coolman1232004/komari-classic-agent:v1.2.5-fix2-security.3.1@sha256:1565d886afe534a8a9961eab32651d0f63b5b568602b9a58aa11c6199da2f130 \
  --disable-web-ssh
```

For Compose, set `env_file: ["./agent.env"]` and `command: ["--disable-web-ssh"]` on the agent service using the same image. Environment variables and an optional JSON config can override flags; do not supply conflicting remote-control settings. Docker administrators can read container environment values, so an env file does not protect tokens from Docker/root access.

For a source build:

```sh
docker build -f agent/Dockerfile.source -t komari-classic-agent:1.2.5-fix2-hardening agent
docker run -d --name komari-agent --restart unless-stopped \
  komari-classic-agent:1.2.5-fix2-hardening \
  -e https://YOUR-KOMARI-HOST -t YOUR-NODE-TOKEN --disable-web-ssh
```

Prefer the env-file approach above to avoid putting a real token in shell history. Use an ordinary Docker network/service address when testing a separate server container. Sharing its network namespace with `--network container:<server>` couples their lifecycle; restart/recreate the agent when the server's network namespace changes.

Container metrics depend on host mounts and namespaces. The command above does not grant host-level monitoring or host terminal access. Add only the access needed for your deployment. All Classic agents now omit the binary self-updater and its deprecated OpenPGP dependency; rebuild the image and recreate the container deliberately. Non-container installations also require manual replacement.

## Backup and rollback

Before changing versions, stop the server and copy the entire `data` directory. Record the running image ID using `docker inspect komari --format '{{.Image}}'` and save the image using `docker image save`. Start the server again after the copy finishes. Keep backups private because they contain credentials and node tokens.

To roll back, stop the server, restore the matching data backup and use the saved previous image. Do not point an older version at a newer-version database without a matching backup. Successful password verification upgrades old password hashes to Argon2id; old binaries cannot verify these new hashes. Avoid unattended `latest` image replacement. After a tested build, deploy the recorded image ID/digest or an immutable release tag.

Password login allows 10 attempts per account per five minutes, 20 per client IP per minute and 60 per server per minute, including 2FA attempts. On HTTP 429, wait for the Retry-After interval. Limits reset when the server process restarts. By default the client IP is the socket peer, so reverse-proxy users share that budget. Security 3 can use an explicit `KOMARI_TRUSTED_PROXIES` configuration; see [the proxy and backup upgrade notes](RELEASE-SECURITY-3.md). Password migration does not change the password or remove 2FA. Use a password reset for dormant accounts that will not log in to migrate.

Application message size is limited to 8 MiB and backup/theme uploads to 512 MiB in this branch. A reverse proxy should also enforce request-size and connection limits. Large backup restores may need a local restore workflow rather than browser upload.

The extended hardening also rejects unsafe ZIP entries and limits expanded archives to 2 GiB / 10,000 files. Nezha compatibility requires a node pre-created in Komari with a valid UUID/token; it no longer accepts unsolicited registrations. Keep that optional plaintext gRPC listener private or protect it with TLS/VPN. Source Docker builds include a patched cloudflared rebuild and Alpine 3.24. See [the extended audit](SECURITY-AUDIT-2026-09.md) for compatibility changes and verification limits.
