# Install

## Production Kubernetes

`coroot-graft` is packaged as a single runtime image that contains:

- `coroot-graft`
- `bering`
- `sheaft`

The recommended production path is the Helm chart in `charts/coroot-graft`.

Published package entrypoints:

- GitHub Releases for cross-platform CLI archives
- `ghcr.io/mb3r-lab/coroot-graft` for the runtime OCI image
- `oci://ghcr.io/mb3r-lab/charts/coroot-graft` for the Helm chart

Example image pull:

```bash
docker pull ghcr.io/mb3r-lab/coroot-graft:v0.1.1
```

Example OCI chart install:

```bash
helm upgrade --install coroot-graft oci://ghcr.io/mb3r-lab/charts/coroot-graft \
  --version 0.1.1 \
  --namespace coroot-graft \
  --create-namespace \
  --set secrets.existingSecret=coroot-graft-secrets
```

Create a secret with the Coroot password and optional webhook secret:

```bash
kubectl create namespace coroot-graft
kubectl -n coroot-graft create secret generic coroot-graft-secrets \
  --from-literal=COROOT_GRAFT_COROOT_PASSWORD='replace-me' \
  --from-literal=COROOT_GRAFT_WEBHOOK_SECRET='replace-me'
```

Install the chart:

```bash
helm upgrade --install coroot-graft ./charts/coroot-graft \
  --namespace coroot-graft \
  --set secrets.existingSecret=coroot-graft-secrets
```

What the chart does:

- mounts `graft.yaml` and analysis files from a ConfigMap
- injects secrets through environment variables
- runs `coroot-graft serve -config /etc/coroot-graft/graft.yaml`
- persists artifacts under `/var/lib/coroot-graft`
- exposes `/metrics`, `/healthz`, `/readyz`, and webhook endpoints
- annotates the pod so Coroot cluster-agent can scrape custom metrics

## Local Coroot Stack

For local validation with pinned versions:

```bash
export AUTH_BOOTSTRAP_ADMIN_PASSWORD=secret
export COROOT_GRAFT_COROOT_PASSWORD=secret
docker compose -f deploy/docker/coroot-compose.yaml -f deploy/docker/coroot-compose.graft-local.yaml up -d --build
docker compose -f deploy/docker/demo-compose.yaml up -d
```

This starts:

- a pinned local Coroot stack
- a local `coroot-graft` runtime
- a tiny demo topology: `frontend -> checkout`
- a `loadgen` loop that keeps traffic flowing through `/checkout`

Why the extra local compose overlay exists:

- Kubernetes uses Coroot custom-metrics discovery via pod annotations.
- Local Docker has no equivalent discovery path.
- The overlay therefore adds an explicit Prometheus scrape target for `coroot-graft:8095` so the managed Coroot dashboard can render `coroot_graft_*` metrics.

## Configuration Notes

The config loader expands environment variables before YAML parsing, so secrets can be referenced like:

```yaml
coroot:
  password: "${COROOT_GRAFT_COROOT_PASSWORD}"
```

The same pattern works for per-project webhook secrets and any other sensitive values that should not live in Git.
