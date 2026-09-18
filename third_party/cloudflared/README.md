# cloudflared dependency overlay

Docker builds official cloudflared 2026.9.1 source at commit `f11dea9cb7079e90a982c1a2d5548ab40847fdcf`, verified against SHA-256 `d9c67e530861b212529fe67f4d7fe04335dc80f07026370a57c46fa611f7730f` of its codeload tarball.

The release binary carried vulnerable x/crypto 0.53.0 and gRPC 1.83.0 dependencies. These manifests upgrade them to x/crypto 0.56.0 and gRPC 1.83.2 plus required x/net, x/sys, x/term and x/text versions. The compression module is also updated to 1.18.7 for GO-2026-5841; source analysis did not find use of the affected s2 package. Application source is unchanged. Build with Go 1.26.8 and `-mod=readonly`; do not use the upstream vendor directory. Version output identifies this as a Classic security rebuild. Self-update is disabled for this package-managed build.

Upstream: https://github.com/cloudflare/cloudflared/tree/f11dea9cb7079e90a982c1a2d5548ab40847fdcf (Apache-2.0). Its license is copied into the final image. The source archive and module manifests are pinned; Alpine package repositories and base-image tags can receive security updates.

Security 2 upgrades the OpenTelemetry SDK, trace exporter and related modules to 1.45.0, fixing CVE-2026-81870 (GHSA-8wmf-6v46-5gfg). Required transitive modules are locked in go.mod/go.sum. Both Docker build paths run the upstream tracing tests before compiling cloudflared, and both architectures check that the executable starts. A live Cloudflare Tunnel connection requires deployment credentials and is not tested by this repository's CI.

GO-2026-5932 covers the unmaintained x/crypto/openpgp package and has no fixed version. The module is still required for other cryptographic packages; dependency scans must confirm that OpenPGP is not imported. Keep this module-level finding visible in container reports rather than suppressing it.
