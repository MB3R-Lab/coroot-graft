# Versioning

`coroot-graft` follows semantic versioning for its own release number.

Each release also carries a pinned compatibility baseline for:

- Coroot server and agents
- Bering
- Sheaft

Those pins are release artifacts, not suggestions:

- [compatibility-manifest.json](/compatibility-manifest.json)
- [versions/toolchain.env](/versions/toolchain.env)

Implications:

- Bumping Coroot, Bering, or Sheaft is a deliberate release change.
- Coroot runtime images are pinned as immutable digests, not floating tags.
- CI and release workflows must use the pinned refs.
- The production image must embed the same pinned `bering` and `sheaft` binaries.
