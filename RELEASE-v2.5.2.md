## cluster-utils-api v2.5.2 — Swagger dark skin + Try-it-out host

Polish release: usable dark Swagger UI and clear control over which host “Try it out” hits.

### Swagger UI skin
- Custom **dark theme by default** (stock Swagger has no first-class dark mode — we inject CSS)
- Donkeyx / terminal flair: shell-prompt topbar, phosphor code, scan grid
- `?theme=light` restores stock bright UI

### Try-it-out host / scheme
Swagger defaults to the host you opened. When that is wrong (port-map, ingress, in-cluster Service):

**Query params (most common — bookmark the URL):**
```text
/api-docs/index.html?host=my-svc.ns.svc:8080&scheme=http
/api-docs/index.html?host=api.example.com&scheme=https
```

**Or once at startup:**
| Env | Purpose |
|-----|---------|
| `SWAGGER_HOST` | Default host:port for Try-it-out |
| `SWAGGER_SCHEME` | `http` or `https` |

**Priority:** query → env → request Host / `X-Forwarded-Proto`

The Swagger **info panel shows the live target** (`scheme://host/`), where it came from, and how to override it.

### Docs
- Links to **this repo** (docs/source) and **cluster-utils** (pair toolkit)
- README + k8s sample updated for host override and image `2.5.2`

### Images
- `ghcr.io/donkeyx/cluster-utils-api:2.5.2` / `:latest`
- `donkeyx/cluster-utils-api:2.5.2` / `:latest`
