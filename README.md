# cluster-utils-api

## description

HTTP side of the **cluster-utils** toolkit. Where [cluster-utils](https://github.com/donkeyx/cluster-utils) is the shell box you exec into, this is the **service you drop into an environment** to exercise the platform around it.

Throw it into a namespace / ECS task / compose stack and use it to test:

- **probes** — real kube-style `/startupz`, `/livez`, `/readyz` with fail + delay + flap + runtime control
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

## Auth (how to get the token)

Most routes are open. Anything under **`/a/`** needs a bearer token:

```http
Authorization: Bearer <token>
```

That covers:

- `GET /a/env` — dump process environment
- `GET/PUT /a/control/probes` — read/flip probe modes without redeploying

### Option 1 — fixed token (easiest for demos)

```bash
docker run -d -p 8080:8080 -e AUTH_TOKEN=dev --name test-api donkeyx/cluster-utils-api:latest
export TOKEN=dev
```

Same idea in k8s — set `AUTH_TOKEN` on the container env.

### Option 2 — random token from logs (default)

If you **don’t** set `AUTH_TOKEN`, a random token is generated **every process start** and printed in the logs (JSON).

Look for fields like `token` / `header`, or grep:

```bash
# local docker
docker logs test-api 2>&1 | grep -E 'token|Bearer|example curl' | head

# pull just the token value out of the json line (if jq + logs are one json object per line)
docker logs test-api 2>&1 | grep '"token"' | tail -1 | jq -r '.token'

# kubernetes
kubectl -n default logs deploy/cluster-utils-api --tail=50 | grep -E 'token|Bearer|example curl'
```

On startup the app also logs **ready-made curls** (env dump + probe control) with the token already filled in — copy/paste those.

### Using it

```bash
export TOKEN=dev   # or whatever you pulled from logs

curl -sS -H "Authorization: Bearer $TOKEN" localhost:8080/a/env | jq
curl -sS -H "Authorization: Bearer $TOKEN" localhost:8080/a/control/probes | jq
```

### Swagger UI

1. Open the UI **on the same host/port you want to call** (that way Try it out just works):
   - local: http://localhost:8080/ or `/api-docs/index.html`
   - port-forward: http://localhost:8080/api-docs/index.html
2. Click **Authorize** (lock icon)
3. Value: `Bearer <token>` — word **Bearer**, a space, then the token  
   Example: `Bearer dev`  
   Header name is `Authorization`. The UI remembers it in this browser (`PersistAuthorization`).
4. **Try it out** on any route — protected ones under `/a/` need Authorize first.

**Host / port for Try it out**

Swagger uses the host you opened the page on (so docker `:8080`, port-forward, or in-cluster ingress all line up without editing the spec).

If you ever need to point Try it out somewhere else (UI on A, API on B):

```text
http://localhost:8080/api-docs/index.html?host=cluster-utils-api-svc:8080&scheme=http
http://localhost:8080/api-docs/index.html?host=my-alb.example.com&scheme=https
```

`host` = `hostname` or `hostname:port` (no `http://`). `scheme` = `http` or `https`.

Without a token, `/a/*` returns **401**.

---

## Quick start

```bash
docker run -d -p 8080:8080 -e AUTH_TOKEN=dev --name test-api donkeyx/cluster-utils-api:latest
export TOKEN=dev

curl -sS localhost:8080/help | jq
curl -sS localhost:8080/version | jq
curl -sS localhost:8080/livez
curl -sS -H "Authorization: Bearer $TOKEN" localhost:8080/a/control/probes | jq
```

Port defaults to `8080` (`PORT` to override).

---

## Examples (debugging recipes)

Assume `TOKEN` is set (see **Auth** above) and the api is on `localhost:8080`.

### 1. What did the ingress / mesh actually send me?

```bash
# headers as the pod saw them (X-Forwarded-*, cookies, auth, host, …)
curl -sS -H 'X-Request-Id: demo-1' -H 'X-Forwarded-For: 1.2.3.4' \
  localhost:8080/headers | jq

# fuller dump: hostname, client ip, uri, all headers
curl -sS localhost:8080/debug | jq

# bounce method + body + query back (good for POST through a gateway)
curl -sS -X POST 'localhost:8080/echo?from=ingress' \
  -H 'Content-Type: application/json' \
  -d '{"hello":"cluster"}' | jq
```

### 2. Did my ConfigMap / Secret / task def actually land?

```bash
curl -sS -H "Authorization: Bearer $TOKEN" localhost:8080/a/env | jq
# or one key:
curl -sS -H "Authorization: Bearer $TOKEN" localhost:8080/a/env | jq '."MY_FEATURE_FLAG"'
```

### 3. Readiness: pull the pod out of the Service (no restart)

```bash
# fail ready → kube should remove endpoints; process stays up
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"ready":{"mode":"fail"}}' localhost:8080/a/control/probes | jq

curl -sS -o /dev/null -w 'readyz=%{http_code}\n' localhost:8080/readyz
# in cluster: kubectl get endpoints cluster-utils-api-svc -w

# put it back
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"ready":{"mode":"ok"}}' localhost:8080/a/control/probes | jq
```

### 4. Liveness: make kube restart the container

```bash
# careful — this will restart once failureThreshold is hit
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"live":{"mode":"fail"}}' localhost:8080/a/control/probes | jq

# watch
# kubectl get pod -l type=api -w
```

### 5. Probe timeouts (slow answers)

Sample manifest uses `timeoutSeconds: 1`. Anything slower counts as a failed probe.

```bash
# ready answers after 3s → timeouts with timeoutSeconds: 1
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"ready":{"mode":"delay","delaySeconds":3}}' localhost:8080/a/control/probes | jq

time curl -sS -o /dev/null -w '%{http_code}\n' localhost:8080/readyz
```

### 6. Flapping readiness (in/out of endpoints)

```bash
# time based: 5s ok, 5s fail, repeat
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"ready":{"mode":"flap","flapSeconds":5}}' localhost:8080/a/control/probes | jq

# or every 2nd request fails (handy from a loop)
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"ready":{"mode":"flap","flapEvery":2}}' localhost:8080/a/control/probes | jq

for i in 1 2 3 4; do curl -sS -o /dev/null -w "$i %{http_code}\n" localhost:8080/readyz; done

# see which half of the flap you're in
curl -sS -H "Authorization: Bearer $TOKEN" localhost:8080/a/control/probes \
  | jq '{ready, readyFlapPhase, uptimeSeconds}'
```

### 7. Slow startup / cold start

```bash
# pretend the app needs 15s to init, then latch "started"
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"startup":{"mode":"ok","bootDelaySeconds":15},"resetStartupLatch":true}' \
  localhost:8080/a/control/probes | jq

curl -sS -o /dev/null -w 'startupz=%{http_code}\n' localhost:8080/startupz
# wait, then:
curl -sS -o /dev/null -w 'startupz=%{http_code}\n' localhost:8080/startupz
```

Deploy-time without control API:

```bash
docker run -d -p 8080:8080 \
  -e AUTH_TOKEN=dev \
  -e STARTUP_BOOT_DELAY=20 \
  donkeyx/cluster-utils-api:latest
```

### 8. Upstream returns 502 / 503 / 418

```bash
curl -sS -o /dev/null -w '%{http_code}\n' localhost:8080/status/502
curl -sS -o /dev/null -w '%{http_code}\n' localhost:8080/status/503
curl -sS -o /dev/null -w '%{http_code}\n' localhost:8080/status/418
```

### 9. Which build is this pod?

```bash
curl -sS localhost:8080/version | jq
# {"version":"...","gitHash":"...","hostname":"..."}
```

### 10. From inside the cluster (with cluster-utils shell)

```bash
# port-forward
kubectl -n default port-forward svc/cluster-utils-api-svc 8080:8080

# or exec into the toolkit image and curl the service DNS
kubectl exec -it deploy/cluster-utils -- \
  curl -sS http://cluster-utils-api-svc:8080/debug | jq

# token from api pod logs
TOKEN=$(kubectl -n default logs deploy/cluster-utils-api --tail=100 \
  | grep '"token"' | tail -1 | jq -r '.token')
kubectl exec -it deploy/cluster-utils -- \
  curl -sS -H "Authorization: Bearer $TOKEN" http://cluster-utils-api-svc:8080/a/env | jq
```

---

## Probes (reference)

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

### Modes

| mode | after optional delay |
|------|----------------------|
| `ok` | 200 |
| `fail` | 503 |
| `delay` | 200 (use `delaySeconds` > probe `timeoutSeconds` for **timeouts**) |
| `flap` | alternates ok/fail — `flapSeconds` (time half-period, default 5) or `flapEvery` (every Nth request) |

On startup, flap only applies **before** the latch.

### Seed from env (steady state at deploy)

| env | default | notes |
|-----|---------|--------|
| `LIVE_MODE` / `HEALTHY_MODE` / `HEALTHY` | ok | `fail` / `flap` / `delay` |
| `LIVE_DELAY` / `HEALTHY_DELAY` | 0 | |
| `LIVE_FLAP_SECONDS` / `LIVE_FLAP_EVERY` | | flap knobs |
| `READY_MODE` / `READY` | ok | |
| `READY_DELAY` | 0 | |
| `READY_FLAP_SECONDS` / `READY_FLAP_EVERY` | | |
| `STARTUP_MODE` / `STARTUP` | ok | |
| `STARTUP_DELAY` | 0 | per-request sleep while not latched |
| `STARTUP_BOOT_DELAY` | 0 | wall clock before first success |
| `STARTUP_FLAP_SECONDS` / `STARTUP_FLAP_EVERY` | | flap before latch |

Query `?ok=0` still works on live/ready for quick curl hacks — **kube will never send that**, so use env or `/a/control/probes` for real demos.

### Control API (no redeploy)

```bash
curl -sS -H "Authorization: Bearer $TOKEN" localhost:8080/a/control/probes | jq

curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{
    "ready":   {"mode":"flap","flapSeconds":5},
    "live":    {"mode":"ok"},
    "startup": {"mode":"ok","bootDelaySeconds":5},
    "resetStartupLatch": true
  }' localhost:8080/a/control/probes | jq
```

---

## Endpoints

| path | notes |
|------|--------|
| `GET /` | redirect to swagger |
| `GET /api-docs/*` | swagger ui |
| `GET /help` | json list of routes |
| `GET /version` | version + git hash + hostname |
| `GET /startupz` `/startup` | startup probe (latch) |
| `GET /livez` `/healthz` `/health` | liveness |
| `GET /readyz` `/ready` | readiness |
| `GET /ping` | `PONG` (not a kube probe) |
| `GET /headers` | request headers |
| `GET /debug` | hostname / ip / headers / uri |
| `GET /metrics` | prometheus |
| `GET /status/:code` | respond with that http status (100-599) |
| `GET /delay/:seconds` | sleep then 200 |
| `ANY /echo` | bounce method / query / headers / body |
| `GET /a/env` | env vars — **auth** |
| `GET/PUT /a/control/probes` | probe state — **auth** |

### other config

| env | default | what it does |
|-----|---------|----------------|
| `PORT` | `8080` | listen port |
| `AUTH_TOKEN` | random each start | fixed bearer for `/a/*` if set |

---

## Run on Kubernetes

Starts a deployment + ClusterIP service only. If you want a LoadBalancer, wire it yourself — I don't want you to get a bill from this.

```bash
kubectl -n default apply -f \
  https://raw.githubusercontent.com/donkeyx/cluster-utils-api/master/k8s-cluster-util-apis.yml

kubectl get pods,svc -n default
# service: cluster-utils-api-svc:8080
```

Sample manifest probes:

- **startupProbe** → `/startupz` (timeout 1s, period 2s)
- **livenessProbe** → `/livez` (timeout 1s)
- **readinessProbe** → `/readyz` (timeout 1s)

Uncomment the env examples in the yaml to break things on purpose, or flip live via `/a/control/probes` after you grab the token from pod logs (or set `AUTH_TOKEN`).

```bash
kubectl -n default port-forward svc/cluster-utils-api-svc 8080:8080
```
