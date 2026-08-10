# cluster-utils-api

## description

Simple docker image which will stand up a flexibile api that handles most entrypoints and has all the health variations. This allows me to deploy to a cluster, ecs/eks with any entrypoint or params and it will still run and respond to health checks. Great for testing cluster setup and has endpoints for debugging routing and headers.

Default route redirects into the **swagger** docs so you can poke the endpoints from the browser.

| dockerhub: https://hub.docker.com/r/donkeyx/cluster-utils-api

| ghcr: `ghcr.io/donkeyx/cluster-utils-api`

| github: https://github.com/donkeyx/cluster-utils-api

## Usage

Most endpoints are open. Anything under `/a/` is authenticated — grab the bearer token from the container logs on startup (it rotates every restart). The app also logs a ready made curl for `/a/env`.

Swagger UI:

- http://localhost:8080/  (redirects)
- http://localhost:8080/api-docs/index.html

Port defaults to `8080`, override with `PORT` if you need to.

### Start container:

```bash
docker run -d -p 8080:8080 --name test-api donkeyx/cluster-utils-api:latest
```

```bash
# quick route map
curl -sS localhost:8080/help | jq

# health / ping
curl -sS localhost:8080/healthz
curl -sS localhost:8080/ping

# debug routing / headers
curl -sS localhost:8080/headers | jq
curl -sS localhost:8080/debug | jq

# env dump (needs the token from logs)
docker logs test-api 2>&1 | head -30
curl -sS -H "Authorization: Bearer <token>" localhost:8080/a/env | jq
```

`/help` looks roughly like:

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

### Main endpoints

| path | notes |
|------|--------|
| `GET /` | redirect to swagger |
| `GET /api-docs/*` | swagger ui |
| `GET /help` | json list of routes |
| `GET /health` `/healthz` | returns `OK` |
| `GET /ready` `/readyz` | returns `Ready` |
| `GET /ping` | `PONG` |
| `GET /headers` | request headers |
| `GET /debug` | hostname / ip / headers / uri |
| `GET /a/env` | env vars, **bearer auth** |

### run image in k8 cluster:

You can run the pod in your cluster with the commands below. This will start a deployment and service but limited to cluster ip. If you want to expose with type loadbalancer you can do it yourself, I don't want you to get a bill from this.

```bash
# apply pod config
kubectl -n default \
    apply -f https://raw.githubusercontent.com/donkeyx/cluster-utils-api/master/k8s-cluster-util-apis.yml
```

```bash
kubectl get pods,svc -n default
# service is cluster-utils-api-svc on 8080
```

### Now you can use port forwarding to curl your apis inside the cluster

```bash
# in one window forward the ports to the service
kubectl -n default port-forward svc/cluster-utils-api-svc 8080:8080

# then curl the service -> pod
curl -sS localhost:8080/debug | jq
```

Probes in the manifest hit `/healthz` on container port 8080.
