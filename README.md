# cluster-utils-api

## description

HTTP side of the **cluster-utils** toolkit. Where [cluster-utils](https://github.com/donkeyx/cluster-utils) is the shell box you exec into, this is the **service you drop into an environment** to exercise the platform around it.

Throw it into a namespace / ECS task / compose stack and use it to test:

- **probes** — liveness / readiness style paths (`/health`, `/healthz`, `/ready`, `/readyz`, `/ping`)
- **routing & ingress** — hit it through a service, ingress, ALB, mesh; see what actually arrives
- **headers & identity** — what the proxy rewrote, client IP, host, path (`/headers`, `/debug`, `/echo`)
- **config / params in the env** — dump process env behind auth (`/a/env`) so you can check secrets, configmaps, task defs actually landed
- **bad / slow upstreams** — force status codes and delays (`/status/503`, `/delay/5`)
- **any entrypoint noise** — binary is also linked as `node` / `npm` so broken charts that call weird commands still come up and serve the api

More endpoints will keep landing here as we need them. Default route dumps you into **swagger** so you can poke things from the browser without memorising paths.

| dockerhub: https://hub.docker.com/r/donkeyx/cluster-utils-api

| ghcr: `ghcr.io/donkeyx/cluster-utils-api`

| github: https://github.com/donkeyx/cluster-utils-api

| pair with: https://github.com/donkeyx/cluster-utils (shell / toolkit image)

## Usage

Most endpoints are open. Anything under `/a/` is authenticated — grab the bearer token from the container logs on startup (it rotates every restart unless you set `AUTH_TOKEN`). The app also logs a ready made curl for `/a/env`.

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

# what binary is this
curl -sS localhost:8080/version | jq

# health / ping
curl -sS localhost:8080/healthz
curl -sS localhost:8080/ping

# force probes to fail (query wins over env)
curl -sS -o /dev/null -w '%{http_code}\n' 'localhost:8080/ready?ok=0'
curl -sS -o /dev/null -w '%{http_code}\n' 'localhost:8080/healthz?healthy=false'

# status / delay / echo — classic "is my ingress dumb" toolkit
curl -sS -o /dev/null -w '%{http_code}\n' localhost:8080/status/418
curl -sS localhost:8080/delay/1
curl -sS -X POST -d '{"hi":1}' localhost:8080/echo | jq

# debug routing / headers
curl -sS localhost:8080/headers | jq
curl -sS localhost:8080/debug | jq

# prometheus
curl -sS localhost:8080/metrics | head

# env dump (needs the token from logs, or AUTH_TOKEN you set)
docker logs test-api 2>&1 | head -30
curl -sS -H "Authorization: Bearer <token>" localhost:8080/a/env | jq
```

### Main endpoints

| path | notes |
|------|--------|
| `GET /` | redirect to swagger |
| `GET /api-docs/*` | swagger ui |
| `GET /help` | json list of routes |
| `GET /version` | version + git hash + hostname |
| `GET /health` `/healthz` | liveness; fail with `HEALTHY=false` or `?ok=0` |
| `GET /ready` `/readyz` | readiness; fail with `READY=false` or `?ok=0` |
| `GET /ping` | `PONG` |
| `GET /headers` | request headers |
| `GET /debug` | hostname / ip / headers / uri |
| `GET /metrics` | prometheus |
| `GET /status/:code` | respond with that http status (100-599) |
| `GET /delay/:seconds` | sleep then 200 (capped at 30s) |
| `ANY /echo` | bounce method / query / headers / body |
| `GET /a/env` | env vars, **bearer auth** |

### config knobs

| env | default | what it does |
|-----|---------|----------------|
| `PORT` | `8080` | listen port |
| `AUTH_TOKEN` | random each start | fixed bearer token if set |
| `HEALTHY` | true | liveness; `false`/`0` → 503 on /health* |
| `READY` | true | readiness; `false`/`0` → 503 on /ready* |

Query overrides env for a single request: `?ok=0`, `?healthy=false`, `?ready=false`.

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

Manifest has liveness on `/healthz`, readiness on `/readyz`, startup on `/healthz` (port 8080). Flip `READY`/`HEALTHY` in the deployment if you want to watch the probes react.
