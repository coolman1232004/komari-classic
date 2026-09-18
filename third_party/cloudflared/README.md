# cloudflared dependency overlay

Docker builds official cloudflared 2026.9.1 source at commit `f11dea9cb7079e90a982c1a2d5548ab40847fdcf`, verified against SHA-256 `d9c67e530861b212529fe67f4d7fe04335dc80f07026370a57c46fa611f7730f` of its codeload tarball.

The release binary carried vulnerable x/crypto 0.53.0 and gRPC 1.83.0 dependencies. These manifests upgrade them to x/crypto 0.56.0 and gRPC 1.83.2 plus required x/net, x/sys, x/term and x/text versions. The compression module is also updated to 1.18.7 for GO-2026-5841; source analysis did not find use of the affected s2 package. Application source is unchanged. Build with Go 1.26.8 and `-mod=readonly`; do not use the upstream vendor directory. Version output identifies this as a Classic security rebuild. Self-update is disabled for this package-managed build.

Upstream: https://github.com/cloudflare/cloudflared/tree/f11dea9cb7079e90a982c1a2d5548ab40847fdcf (Apache-2.0). Its license is copied into the final image. The source archive and module manifests are pinned; Alpine package repositories and base-image tags can receive security updates.

Both Docker architectures check that the executable starts. A live Cloudflare Tunnel connection requires deployment credentials and is not tested by this repository's CI.
