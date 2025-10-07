# observer

A tiny Kubernetes controller that watches **EndpointSlices** and mirrors the set of **ready pods behind each Service** into a PostgreSQL table. Useful for allowlists, inventories, diagnostics, and simple service discovery glue.

* **Inputs:** `discovery.k8s.io/v1 EndpointSlice` (read-only)
* **Output table (minimal):** `cluster, namespace, service, pod_uid, pod_name, pod_ip, ready, first_seen, last_seen`
* **What it does:** upserts current ready endpoints and prunes stale ones per `{cluster,namespace,service}`

> No leader election, no metrics server, and runs as non-root.

---

## How it works (quick)

* Watches `EndpointSlice` events via `controller-runtime`.
* Filters by optional `ENDPOINT_SELECTOR` (label selector on EndpointSlice).
* For each ready endpoint, **UPSERT** one row (by PK) and set `last_seen=now()`.
* After a sync, **DELETE** any rows for that `{cluster,namespace,service}` not in the current set.

---

## Table schema

Create this once in your DB (you can use a different schema/table name and set `TABLE_NAME` accordingly):

```sql
CREATE TABLE IF NOT EXISTS public.test_server (
  cluster     text        NOT NULL,
  namespace   text        NOT NULL,
  service     text        NOT NULL,
  pod_uid     text        NOT NULL,
  pod_name    text,
  pod_ip      inet        NOT NULL,
  ready       boolean     NOT NULL DEFAULT true,
  first_seen  timestamptz NOT NULL DEFAULT now(),
  last_seen   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (cluster, namespace, service, pod_uid)
);

CREATE INDEX IF NOT EXISTS server_ns_svc ON public.test_server(namespace, service);
CREATE INDEX IF NOT EXISTS server_pod_ip ON public.test_server(pod_ip);
```

---

## Build & Run locally

### Prereqs

* Go **1.24** (or whatever your `go.mod` toolchain is)
* Access to a cluster (`KUBECONFIG`) if you want to run against live EndpointSlices
* A PostgreSQL instance you can reach

### Env vars

The app reads Postgres and behavior from env/flags:

| Variable            | Required | Default         | Notes                                                                              |
| ------------------- | -------- | --------------- | ---------------------------------------------------------------------------------- |
| `PGHOST`            | ✅        | —               | DB host                                                                            |
| `PGPORT`            |          | `5432`          | DB port                                                                            |
| `PGUSER`            | ✅        | —               | DB user                                                                            |
| `PGPASSWORD`        | ✅        | —               | DB password                                                                        |
| `PGDATABASE`        | ✅        | —               | DB name                                                                            |
| `PGSSLMODE`         |          | `require`       | `disable` for local                                                                |
| `ENDPOINT_SELECTOR` |          | *(empty)*       | Label selector on **EndpointSlice** (e.g. `kubernetes.io/service-name=my-service`) |
| `NAMESPACE`         |          | *(empty)*       | If set, watch only this namespace                                                  |
| `TABLE_NAME`        |          | `public.server` | Schema-qualified allowed                                                           |
| `CLUSTER_NAME`      |          | `default`       | Written into `cluster` column                                                      |

Flag equivalents:

* `--requeue-after=30s` (periodic reconcile)
* `--selector`, `--namespace`, `--table`, `--cluster`

### Run

```bash
export PGHOST=127.0.0.1 PGPORT=5432 PGUSER=observer PGPASSWORD=secret PGDATABASE=infra PGSSLMODE=disable
export ENDPOINT_SELECTOR="kubernetes.io/service-name=my-service"
export TABLE_NAME="public.test_server"
export CLUSTER_NAME="dev-cluster"

go run ./cmd/observer --requeue-after=30s
```

---

## Docker

`Dockerfile` builds a static binary and sets version via build arg.

```bash
docker build -t ealebed/observer:latest --build-arg VERSION=0.1.0 .
```

Run (example):

```bash
docker run --rm \
  -e PGHOST=host.docker.internal -e PGPORT=5432 -e PGUSER=observer -e PGPASSWORD=secret -e PGDATABASE=infra -e PGSSLMODE=disable \
  -e ENDPOINT_SELECTOR="kubernetes.io/service-name=my-service" \
  -e TABLE_NAME="public.server" \
  -e CLUSTER_NAME="dev-cluster" \
  ealebed/observer:latest
```

---

## Kubernetes (manifests)

A minimal working set lives in `manifests/observer.yaml`:

* Namespace, ServiceAccount
* ClusterRole/Binding for `endpointslices` read
* Deployment for `observer` (nonroot, read-only FS)

**Important:** EndpointSlices aren’t labeled with your Pod labels; use:

```
ENDPOINT_SELECTOR="kubernetes.io/service-name=my-service"
```

### Smoke-test app

This will create EndpointSlices your observer can see:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  namespace: default
spec:
  selector:
    app: my-service
  ports:
    - port: 80
      targetPort: 80
      protocol: TCP
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-service
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-service
  template:
    metadata:
      labels:
        app: my-service
    spec:
      containers:
        - name: web
          image: nginx:1.25-alpine
          ports:
            - containerPort: 80
```

Then deploy the observer with `ENDPOINT_SELECTOR="kubernetes.io/service-name=my-service"` and watch rows appear.

---

## CI

Typical GitHub Actions (examples in `.github/workflows/`):

* **CI:** build/vet/test + golangci-lint v2
* **Image:** build & push container image to GAR

For linting (v2 schema), a simple `.golangci.yaml` is used.

> If you need to ignore certain folders, add `linters.exclusions.paths` with regexes.

---

## Troubleshooting

* **No rows written:**

  * Ensure your selector matches EndpointSlices:
    `kubectl get endpointslice -l kubernetes.io/service-name=my-service -A`
* **DB connect errors:**

  * Check `PG*` envs in the Pod; verify `PGSSLMODE` vs your DB.
* **RBAC:**

  * Controller needs `get/list/watch` on `discovery.k8s.io/EndpointSlice` (cluster-wide if watching all namespaces).

---

## Development

* Format & lint:

  ```bash
  go fmt ./...
  golangci-lint run --timeout 4m
  ```
* Build:

  ```bash
  go build ./cmd/observer
  ```

---

## Roadmap (nice-to-have)

* Health/readiness endpoints (if you want probes)
* Metrics export (Prometheus) if you later need observability
* Optional Pod enrichment (node name/IP, labels) with additional RBAC
