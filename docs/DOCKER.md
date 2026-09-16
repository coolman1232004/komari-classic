# Docker installation from your fork

The unchanged baseline remains on `main` until the hardening branch is reviewed. For the hardened source build:

```sh
git clone --branch codex/security-hardening https://github.com/coolman1232004/komari-classic.git
cd komari-classic
docker compose up -d --build
docker compose logs komari
```

The server binds to `127.0.0.1:25774` by default. Put an HTTPS reverse proxy in front of it, or access it through an SSH tunnel. To expose the port directly, set `KOMARI_BIND_ADDRESS=0.0.0.0` in `.env` before starting. The proxy must overwrite forwarded scheme headers and forward WebSocket upgrades. Keep API/WebSocket origin checking enabled.

Initial administrator credentials appear in startup logs. Log in, choose a unique password and enable 2FA. Data stays in `./data`. Do not mount the Docker socket or use privileged mode for the server. The Compose configuration drops capabilities, prevents privilege escalation and rotates logs.

This builds the UI and server inside Docker; Go and Node are not required on the host. The existing release Dockerfile remains available for prebuilt release binaries. No prebuilt hardened GHCR image is promised until it has actually been published. The source build works independently of GHCR publishing.

## Agent image

```sh
docker build -f agent/Dockerfile.source -t komari-classic-agent:1.2.5-fix2-hardening agent
docker run -d --name komari-agent --restart unless-stopped \
  komari-classic-agent:1.2.5-fix2-hardening \
  -e https://YOUR-KOMARI-HOST -t YOUR-NODE-TOKEN
```

Container metrics depend on host mounts and namespaces. The command above does not grant host-level monitoring or host terminal access. Add only the access needed for your deployment. Container agents do not self-update their executable; rebuild the image and recreate the container deliberately.

## Backup and rollback

Before changing versions, stop the server and copy the entire `data` directory. Record the running image ID using `docker inspect komari --format '{{.Image}}'` and save the image using `docker image save`. Start the server again after the copy finishes. Keep backups private because they contain credentials and node tokens.

To roll back, stop the server, restore the matching data backup and use the saved previous image. Do not point an older version at a newer-version database without a matching backup. Avoid unattended `latest` image replacement. After a tested build, deploy the recorded image ID/digest or an immutable release tag.

Application message size is limited to 8 MiB and backup/theme uploads to 512 MiB in this branch. A reverse proxy should also enforce request-size and connection limits. Large backup restores may need a local restore workflow rather than browser upload.
