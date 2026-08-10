# cluster-utils-api

Flexible test API for clusters (Kubernetes, ECS/EKS, local Docker). It accepts normal health probes, exposes debug helpers for routing/headers, and serves interactive **Swagger** docs.

| | |
|---|---|
| **Docker Hub** | https://hub.docker.com/r/donkeyx/cluster-utils-api |
| **GHCR** | `ghcr.io/donkeyx/cluster-utils-api` |
| **GitHub** | https://github.com/donkeyx/cluster-utils-api |

## Quick start

```bash
docker run -d -p 8080:8080 --name test-api donkeyx/cluster-utils-api:latest
```

Open Swagger UI in a browser:

- http://localhost:8080/ → redirects to the docs  
- http://localhost:8080/api-docs/index.html

Default listen port is **8080**. Override with env `PORT`.

## Swagger & help

Interactive OpenAPI (Swagger 2.0) is built into the image. Prefer the UI for request/response detail.

Quick machine-readable route list:

```bash
curl -sS localhost:8080/help | jq
```

Example:

```json
{
  "/": "This can be used to redirect to the swagger docs for more details",
  "/a/env": "GET",
  "/debug": "GET",
  "/headers": "GET",
  "/health": "GET",
  "/healthz": "GET",
  "/ping": "GET",
  "/ready": "GET",
  "/readyz": "GET"
}
```

## Endpoints

| Path | Auth | Description |
|------|------|-------------|
| `GET /` | no | Redirect to Swagger UI |
| `GET /api-docs/*` | no | Swagger UI + openapi assets |
| `GET /help` | no | JSON map of main routes |
| `GET /health`, `/healthz` | no | Liveness-style health (`OK`) |
| `GET /ready`, `/readyz` | no | Readiness (`Ready`) |
| `GET /ping` | no | Simple `PONG` |
| `GET /headers` | no | Request headers as JSON |
| `GET /debug` | no | Hostname, client IP, headers, URI |
| `GET /a/env` | **Bearer** | Process environment variables |

### Auth (`/a/*`)

Routes under `/a/` require:

```http
Authorization: Bearer <token>
```

On startup the process generates a random token and logs it (and a ready-made curl). Token rotates every restart.

```bash
# token is printed in container logs as "Random Security Token"
docker logs test-api 2>&1 | head -20

curl -sS -H "Authorization: Bearer <token>" localhost:8080/a/env | jq
```

## Examples

```bash
curl -sS localhost:8080/healthz
# OK

curl -sS localhost:8080/ping
# PONG

curl -sS localhost:8080/headers | jq
curl -sS localhost:8080/debug | jq
```

## Run on Kubernetes

The sample manifest starts a Deployment and ClusterIP Service (no LoadBalancer, so no surprise cloud bill).

```bash
kubectl -n default apply -f \
  https://raw.githubusercontent.com/donkeyx/cluster-utils-api/master/k8s-cluster-util-apis.yml
```

```bash
kubectl get pods,svc -n default -l type=api
kubectl -n default port-forward svc/cluster-utils-api-svc 8080:8080
```

Then use the same local URLs as above (`http://localhost:8080/`, `/debug`, etc.).

Manifest probes hit `/healthz` on container port **8080**. Image pull defaults to `donkeyx/cluster-utils-api:latest`.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |

## Images

Multi-arch (`linux/amd64`, `linux/arm64`) images are published to Docker Hub and GHCR on tagged releases and default-branch builds. Pull either:

```bash
docker pull donkeyx/cluster-utils-api:latest
docker pull ghcr.io/donkeyx/cluster-utils-api:latest
```
