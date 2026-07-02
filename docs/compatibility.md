# Compatibility

`coroot-graft` releases are expected to pin and publish the exact upstream versions they were validated against.

Current tested baseline:

- Coroot server image: `ghcr.io/coroot/coroot@sha256:e64c7a1dfa91ed60fe7cd540031f1ed4e3541f26d50b72465ae574b19625819d` (`1.18.8`)
- Coroot node agent image: `ghcr.io/coroot/coroot-node-agent@sha256:ce7b51149332bbfc6cf94ab1a9ca0ea00155b83ad02f68735295b58bfdd41634` (`latest` digest pinned for the compose-compatible collector contract)
- Coroot cluster agent image: `ghcr.io/coroot/coroot-cluster-agent@sha256:ebeed930e00ceb7c3d577a8f371c541cc409457830c5e67b237dce350c7c4250` (`1.6.1`)
- Prometheus image: `prom/prometheus@sha256:7a34573f0b9c952286b33d537f233cd5b708e12263733aa646e50c33f598f16c` (`v2.53.5`)
- ClickHouse image: `clickhouse/clickhouse-server@sha256:85b97f63dcfff47790d26bb5d5801637aaddb2b93e5e9aee27a686c2fb2b9916` (`24.3`)
- Bering release: `v1.0.0`
- Bering ref: `d858f09a8cca8edf302646a54b28412d158c0ec2`
- Sheaft release: `v1.0.0`
- Sheaft ref: `fa40ee250b9dbf7f106ea7a0c49e35fdd5105172`
- Bering model/snapshot contract: `1.3.0`

The single source of truth for these pins is [compatibility-manifest.json](/compatibility-manifest.json) and [versions/toolchain.env](/versions/toolchain.env).

Release policy:

- Each `coroot-graft` release must update the pinned Coroot, Bering, and Sheaft versions deliberately.
- Each `coroot-graft` release must keep Coroot runtime images immutable by pinning exact image digests.
- CI and release workflows must use those pinned refs instead of `main`.
- The production image must embed the pinned `bering` and `sheaft` binaries.
- Production deployment manifests and Helm values must reference the same tested baseline.
