# cluster-utils-api

## description

HTTP side of the **cluster-utils** toolkit. Where [cluster-utils](https://github.com/donkeyx/cluster-utils) is the shell box you exec into, this is the **service you drop into an environment** to exercise the platform around it.

Throw it into a namespace / ECS task / compose stack and use it to test:

- **probes** — real kube-style `/startupz`, `/livez`, `/readyz` with fail + delay + runtime control
- **routing & ingress** — hit it through a service, ingress, ALB, mesh; see what actually arrives
- **headers & identity** — what the proxy rewrote, client IP, host, path (`/headers`, `/debug`, `/echo`)
- **config / params in the env** — dump process env behind auth (`/a/env`) so you can check secrets, configmaps, task defs actually landed
- **bad / slow upstreams** — force status codes and delays (`/status/503`, `/delay/5`)
- **any entrypoint noise** — binary is also linked as `node` / `npm` so broken charts that call weird commands still come up and serve the api

Default route dumps you into **swagger** so you can poke things from the browser without memorising paths.

| dockerhub: https://hub.docker.com/r/donkeyx/cluster-utils-api

| ghcr: `ghcr.io/donkeyx/cluster-utils-api`

| github: https://github.com/donkeyx/cluster-utils-api

| pair with: https://github.com/donkeyx/cluster-utils (shell / toolkit image)

## Usage

Most endpoints are open. Anything under `/a/` is authenticated — grab the bearer token from the container logs on startup (it rotates every restart unless you set `AUTH_TOKEN`). The app also logs ready made curls.

Swagger UI:

- http://localhost:8080/  (redirects)
- http://localhost:8080/api-docs/index.html

Port defaults to `8080`, override with `PORT` if you need to.

### Start container:

```bash
docker run -d -p 8080:8080 --name test-api donkeyx/cluster-utils-api:latest
# fixed token for demos:
# docker run -d -p 8080:8080 -e AUTH_TOKEN=dev donkeyx/cluster-utils-api:latest
```

```bash
curl -sS localhost:8080/help | jq
curl -sS localhost:8080/version | jq

# kube-style probes
curl -sS localhost:8080/startupz
curl -sS localhost:8080/livez
curl -sS localhost:8080/readyz

# force ready fail without redeploy (token from logs)
TOKEN=...   # or AUTH_TOKEN you set
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"ready":{"mode":"fail"}}' localhost:8080/a/control/probes | jq
curl -sS -o /dev/null -w '%{http_code}\n' localhost:8080/readyz

# slow live so kube timeoutSeconds trips
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"live":{"mode":"delay","delaySeconds":3}}' localhost:8080/a/control/probes | jq

# cold start again without restarting the process
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"startup":{"mode":"ok","bootDelaySeconds":10},"resetStartupLatch":true}' \
  localhost:8080/a/control/probes | jq
curl -sS -o /dev/null -w '%{http_code}\n' localhost:8080/startupz

# status / delay / echo
curl -sS -o /dev/null -w '%{http_code}\n' localhost:8080/status/418
curl -sS localhost:8080/delay/1
curl -sS -X POST -d '{"hi":1}' localhost:8080/echo | jq

curl -sS localhost:8080/debug | jq
curl -sS -H "Authorization: Bearer $TOKEN" localhost:8080/a/env | jq
```

## Probes (the main event)

These follow the usual kube split. **Status codes matter more than bodies** — kube only cares 2xx vs not (and timeouts).

| path | kube role | fail means | aliases |
|------|-----------|------------|---------|
| `GET /startupz` | **startupProbe** | still starting / forced fail | `/startup` |
| `GET /livez` | **livenessProbe** | **restart** the container | `/healthz`, `/health` |
| `GET /readyz` | **readinessProbe** | leave **Service endpoints** (process stays up) | `/ready` |

### Startup behaves like a real startup endpoint

1. While `bootDelaySeconds` has not elapsed (from process start, or after a latch reset) → **503** `starting`
2. First success → **latches** to started
3. After latch → always **200** `started` (fast), until process restart or `resetStartupLatch`
4. `mode=fail` → **503** `startup failed` and **never** latches

That matches what kube expects: spam startup until it works, then stop and only run live/ready.

### Modes + delay (live / ready / startup)

| mode | after optional delay |
|------|----------------------|
| `ok` | 200 |
| `fail` | 503 |
| `delay` | 200 (same as ok — use with `delaySeconds` > probe `timeoutSeconds` to force **timeouts**) |

`delaySeconds` is always applied on live/ready (capped at 30s). On startup it applies until latched; once latched answers are immediate.

### Seed from env (steady state at deploy)

| env | default | notes |
|-----|---------|--------|
| `LIVE_MODE` / `HEALTHY_MODE` / `HEALTHY` | ok | `false`/`fail` → live 503 |
| `LIVE_DELAY` / `HEALTHY_DELAY` | 0 | seconds before live answers |
| `READY_MODE` / `READY` | ok | |
| `READY_DELAY` | 0 | |
| `STARTUP_MODE` / `STARTUP` | ok | |
| `STARTUP_DELAY` | 0 | per-request sleep while not latched |
| `STARTUP_BOOT_DELAY` | 0 | wall clock from start before first success allowed |

### Flip at runtime (no redeploy)

Auth required (same bearer as `/a/env`):

```bash
# read
curl -sS -H "Authorization: Bearer $TOKEN" localhost:8080/a/control/probes | jq

# write (partial update)
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{
    "ready":   {"mode":"fail","delaySeconds":0},
    "live":    {"mode":"ok","delaySeconds":0},
    "startup": {"mode":"ok","delaySeconds":0,"bootDelaySeconds":5},
    "resetStartupLatch": true
  }' localhost:8080/a/control/probes | jq
```

Query `?ok=0` still works on live/ready for quick curl hacks — **kube will never send that**, so use env or the control API for real probe demos.

### Other endpoints

| path | notes |
|------|--------|
| `GET /` | redirect to swagger |
| `GET /api-docs/*` | swagger ui |
| `GET /help` | json list of routes |
| `GET /version` | version + git hash + hostname |
| `GET /ping` | `PONG` (not a kube probe) |
| `GET /headers` | request headers |
| `GET /debug` | hostname / ip / headers / uri |
| `GET /metrics` | prometheus |
| `GET /status/:code` | respond with that http status (100-599) |
| `GET /delay/:seconds` | sleep then 200 (generic; prefer probe delays for kube) |
| `ANY /echo` | bounce method / query / headers / body |
| `GET /a/env` | env vars, **bearer auth** |
| `GET/PUT /a/control/probes` | probe state, **bearer auth** |

### other config

| env | default | what it does |
|-----|---------|----------------|
| `PORT` | `8080` | listen port |
| `AUTH_TOKEN` | random each start | fixed bearer token if set |

### run image in k8 cluster:

You can run the pod in your cluster with the commands below. This will start a deployment and service but limited to cluster ip. If you want to expose with type loadbalancer you can do it yourself, I don't want you to get a bill from this.

```bash
kubectl -n default \
    apply -f https://raw.githubusercontent.com/donkeyx/cluster-utils-api/master/k8s-cluster-util-apis.yml
```

```bash
kubectl get pods,svc -n default
# service is cluster-utils-api-svc on 8080
```

Sample manifest uses:

- **startupProbe** → `/startupz` (timeout 1s, period 2s)
- **livenessProbe** → `/livez` (timeout 1s)
- **readinessProbe** → `/readyz` (timeout 1s)

Short timeouts make `delaySeconds: 3` an obvious timeout fail. Uncomment the env examples in the yaml to break things on purpose, or flip them live via `/a/control/probes`.

```bash
kubectl -n default port-forward svc/cluster-utils-api-svc 8080:8080
curl -sS localhost:8080/readyz
```
