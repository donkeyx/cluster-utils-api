# 🐴 DonkeyX's Cluster Utils API

```
╭────────────────────────────────────────╮
|   🐴 DonkeyX's Cluster Utils API      │
╰────────────────────────────────────────╯

        //\\
       (/oo\)   .----.
       (____)  | API |
        /||\   '----'
       //||\\   🔌 Probe Mode
      ^^ ^^ ^^
   "Kick the tyres on the mesh!"
```

## description

HTTP side of the **cluster-utils** toolkit. Where [cluster-utils](https://github.com/donkeyx/cluster-utils) is the **shell box you exec into**, this is the **service you drop into an environment** and hit over HTTP — same donkey energy, different job.

Throw it into a namespace / ECS task / compose stack and use it to test:

- **probes** — real kube-style `/startupz`, `/livez`, `/readyz` with fail + delay + flap + runtime control
- **routing & ingress** — hit it through a service, ingress, ALB, mesh; see what actually arrives
- **east-west hops** — north-south into this pod, then `/proxy` out to another svc (headers ride along)
- **headers & identity** — what the proxy rewrote, client IP, host, path (`/headers`, `/debug`, `/echo`)
- **config / params in the env** — dump process env behind auth (`/a/env`) so you can check secrets, configmaps, task defs actually landed
- **bad / slow upstreams** — force status codes and long delays (`/status/503`, `/delay/90`)
- **any entrypoint noise** — binary is also linked as `node` / `npm` so broken charts that call weird commands still come up and serve the api

Default route dumps you into **swagger** so you can poke things from the browser without memorising paths.

| dockerhub: https://hub.docker.com/r/donkeyx/cluster-utils-api

| ghcr: `ghcr.io/donkeyx/cluster-utils-api`

| github: https://github.com/donkeyx/cluster-utils-api

| pair with: https://github.com/donkeyx/cluster-utils (shell / toolkit image)

## Auth (how to get the token)

Most routes are open on purpose (probes, ingress debug). Anything under **`/a/`** needs a bearer token:

```http
Authorization: Bearer <token>
```

| path | why it's locked |
|------|------------------|
| `GET /a/env` | dumps **all env** — secrets, keys, tokens |
| `GET/PUT /a/control/probes` | can fail live (restarts) / ready (drop traffic) |
| `GET/POST /a/proxy` | **SSRF** if open — scan the cluster, hit metadata, pull internal APIs |

See **Security** below for the full split.

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

### 9. Really slow request

```bash
# sleep then 200 — default cap 120s (override with MAX_DELAY_SECONDS, hard max 600)
curl -sS localhost:8080/delay/90
# delayed=90.000s requested=90.000s max=120s

# or make a *probe* slow (so kube timeoutSeconds trips)
curl -sS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"ready":{"mode":"delay","delaySeconds":30}}' localhost:8080/a/control/probes | jq
```

### 10. East-west hop via north-south (`/a/proxy`) — **auth required**

Pattern: **ingress → this api → another service** (mesh / NetworkPolicy / DNS / header propagation).

Locked behind bearer on purpose: an open proxy is **SSRF** (anyone could make your pod call internal URLs).

#### What you get back (default)

Unless `"raw": true`, the HTTP response from *this* api is always **200 + JSON wrap** (even if upstream was 502). Inside that wrap you get the **full upstream response**:

| field | what it is |
|-------|------------|
| `response.status` | status code from the other API |
| `response.headers` | **all response headers** from the other API |
| `response.body` | body as a string (capped ~2MB in the wrap) |
| `request.url` / `method` / `headers` / `body` | what we actually sent east-west |
| `meta.durationMs` | hop timing |
| `meta.forwardIncomingHeaders` | whether inbound headers were copied |
| `meta.forwardSensitiveHeaders` | whether Authorization/Cookie were copied |

Example shape:

```json
{
  "request": {
    "url": "http://other-api:8080/debug",
    "method": "GET",
    "headers": { "X-Request-Id": ["demo"], "X-Cu-Proxy-Hop": ["pod-a"] },
    "body": ""
  },
  "response": {
    "status": 200,
    "headers": {
      "Content-Type": ["application/json; charset=utf-8"]
    },
    "body": "{\"Hostname\":\"other-pod\", ...}"
  },
  "meta": {
    "durationMs": 12,
    "timeoutSeconds": 10,
    "forwardIncomingHeaders": true,
    "forwardSensitiveHeaders": false,
    "proxyHostname": "edge-pod"
  }
}
```

Handy jq:

```bash
# just upstream status + headers + body
curl -sS -H "Authorization: Bearer $TOKEN" -H 'X-Request-Id: demo' \
  "$BASE/a/proxy?url=http://other-api:8080/debug" \
  | jq '{status: .response.status, headers: .response.headers, body: .response.body}'
```

`"raw": true` → no wrap; you get the upstream status/headers/body as the real HTTP response (harder to inspect the hop).

#### Header forwarding

| inbound headers | default |
|-----------------|---------|
| tracing / custom (`X-Request-Id`, etc.) | **forwarded** |
| hop-by-hop (`Host`, `Connection`, `Content-Length`, …) | stripped |
| **`Authorization` / `Cookie`** | **not** forwarded (so your `/a/*` bearer is not sent to the other svc by accident) |

To forward credentials east-west on purpose: `"forwardSensitiveHeaders": true`, or set `headers.Authorization` in the JSON body.

```bash
export BASE=http://localhost:8080
export TOKEN=dev

# simple GET hop
curl -sS -H "Authorization: Bearer $TOKEN" \
  -H 'X-Request-Id: demo-ew-1' -H 'X-Trace: abc' \
  "$BASE/a/proxy?url=http://other-api:8080/debug" | jq

# POST form — full control
curl -sS -X POST "$BASE/a/proxy" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'X-Request-Id: demo-ew-2' \
  -d '{
    "url": "http://other-api:8080/echo",
    "method": "POST",
    "body": "{\"ping\":true}",
    "headers": {"Content-Type": "application/json"},
    "timeoutSeconds": 15,
    "forwardIncomingHeaders": true
  }' | jq

# upstream headers only
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$BASE/a/proxy?url=http://other-api:8080/headers" | jq '.response.headers'

# slow peer
curl -sS -X POST "$BASE/a/proxy" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"url":"http://other-api:8080/delay/5","timeoutSeconds":30}' | jq '.meta'
```

In-cluster (service DNS) from a port-forwarded edge api:

```bash
curl -sS -H "Authorization: Bearer $TOKEN" -H 'X-Request-Id: from-laptop' \
  "$BASE/a/proxy?url=http://cluster-utils-api-svc.other-ns.svc.cluster.local:8080/debug" | jq
```

Chain multi-hop if you want: A `/a/proxy` → B `/a/proxy` → C `/debug` (each hop needs a token for that api).

### 11. Which build is this pod?

```bash
curl -sS localhost:8080/version | jq
# {"version":"...","gitHash":"...","hostname":"..."}
```

### 12. From inside the cluster (with cluster-utils shell)

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
| `GET /metrics` | prometheus (OpenMetrics): request count/latency/in-flight + Go/process |
| `GET /status/:code` | respond with that http status (100-599) |
| `GET /delay/:seconds` | sleep then 200 (cap `MAX_DELAY_SECONDS`, default 120) |
| `ANY /echo` | bounce method / query / headers / body |
| `GET /a/env` | env vars — **auth** |
| `GET/PUT /a/control/probes` | probe state — **auth** |
| `GET/POST /a/proxy` | east-west hop; full upstream status/headers/body in wrap — **auth** |

### other config

| env | default | what it does |
|-----|---------|----------------|
| `PORT` | `8080` | listen port |
| `AUTH_TOKEN` | random each start | fixed bearer for `/a/*` if set |
| `MAX_DELAY_SECONDS` | `120` (hard max 600) | cap for `/delay`, probe delays, proxy timeouts |

---

## Security

This image is a **cluster debug tool**, not a public SaaS. Treat it like you treat `kubectl` access.

### Behind bearer (`/a/*`) — keep it that way

| endpoint | risk if open |
|----------|----------------|
| **`/a/env`** | Full process env — **secrets, API keys, cloud creds**. Correct to lock. |
| **`/a/control/probes`** | Fail liveness → restarts; fail readiness → blackhole traffic; flap → chaos. |
| **`/a/proxy`** | **SSRF**: call any http(s) URL the pod can reach (other namespaces, cloud metadata `169.254.169.254`, internal admin UIs). Response can **exfiltrate** internal data (status + headers + body) to whoever called you. Also why we do **not** auto-forward `Authorization`/`Cookie` east-west. |

Same auth model as `/a/env` is the right call for `/a/proxy`. If it’s reachable from an ingress without network policy, auth is the main brake.

### Open on purpose (kube + ingress testing)

| endpoint | notes |
|----------|--------|
| `/startupz` `/livez` `/readyz` (+ aliases) | **Must** be unauthenticated — kube probes send no bearer |
| `/ping` `/version` `/help` | low sensitivity |
| `/headers` `/debug` `/echo` | can show request headers (including if a client *sent* a secret). Fine for a debug pod; don’t put internet-wide without a gateway auth layer |
| `/status/*` `/delay/*` | abuse = noisy DoS / long requests; cap delay; don’t expose to the open internet |
| `/metrics` | process metrics — usually ok inside the mesh |
| swagger `/api-docs` | documents everything including how to call `/a/*` |

### Practical guidance

- Prefer **ClusterIP** + port-forward / exec (as in the sample manifest) for day-to-day use  
- If you put it on an ingress, put **auth at the edge** too — don’t rely only on the random token in logs  
- Set **`AUTH_TOKEN`** to something you control when automating; rotate if logs are widely readable  
- `/a/proxy` is powerful: only point `url` at targets you intend; assume the response body may contain sensitive data from the peer  

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
