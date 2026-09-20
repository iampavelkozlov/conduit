# Microservices infrastructure

This directory contains the complete deployment topology. The public RealWorld
contract is owned by the stateless `gateway`; all domain applications are
internal gRPC services.

| Application | Port | Database | Binary |
| --- | ---: | --- | --- |
| gateway | 8000 HTTP | none | `/app/conduit` |
| auth | 9001 gRPC | `conduit_auth` | `/app/auth` |
| profile | 9002 gRPC | `conduit_profile` | `/app/profile` |
| posts | 9003 gRPC | `conduit_posts` | `/app/posts` |
| subscriptions | 9004 gRPC | `conduit_subscriptions` | `/app/subscriptions` |
| comments | 9005 gRPC | `conduit_comments` | `/app/comments` |

Every database has a distinct owner. Sharing the local PostgreSQL process is a
cost-saving development choice, not permission for cross-database queries.
Kafka is available for outbox events and Redis for disposable cache entries;
neither is on the synchronous request path by default.

## Docker Compose

Start only the shared local platform with:

```bash
docker-compose -f deploy/compose/docker-compose.microservices.yml up -d postgres kafka redis
```

Start the complete topology, including the frontend and gRPC-backed gateway,
with:

```bash
make microservices-up
```

The development credentials in the Compose file are intentionally local-only.
The PostgreSQL bootstrap scripts run only when its volume is first created. If
the database list changes, create the missing database manually; deleting the
volume destroys all local data and should only be done deliberately.

Useful host endpoints are frontend `localhost:3000`, gateway `localhost:8000`, Kafka
`localhost:29092`, Redis `localhost:6379`, and PostgreSQL `localhost:5432`.
Service ports `9001` through `9005` are published for `grpcurl` and debugging.

## Kubernetes

`deploy/kubernetes/base` is the production-oriented Kustomize base. It contains:

- three replicas for every stateless application;
- a public `LoadBalancer` Service only for gateway;
- native HTTP/gRPC startup, readiness, and liveness probes;
- PodDisruptionBudgets, HPAs, rolling updates, and topology spreading;
- default-deny NetworkPolicies with explicit application, platform, and DNS
  traffic;
- one development PostgreSQL, Kafka KRaft broker, and Redis instance.

The base uses one `conduit:local` image containing multiple binaries. A
production overlay can replace it with independently versioned service images.
The supported laptop path uses the dedicated kind overlay and Make lifecycle:

```bash
make k8s-check
make k8s-status
make k8s-down
```

`deploy/kubernetes/overlays/kind` scales applications and relays to one replica,
removes HPA resources, and lowers scheduler requests. The production base keeps
its three application replicas, two relay replicas, disruption budgets and
autoscaling policy. Use `make k8s-validate` to render and verify both variants.

The jobs overlay contains one active migration Job per database plus an
idempotent Kafka topic-initialization Job. Apply it during a rollout before
considering the corresponding application ready; each migration Job uses only
that service's `/app/migrations/<service>` directory.

The three replicas and disruption budgets assume a multi-node cluster with
enough allocatable CPU and memory. Do not run the full Compose topology beside
kind on a small Docker Desktop VM: the duplicate Kafka and PostgreSQL stacks can
exhaust its memory.

The checked-in Secret contains development defaults so the base is
self-contained. A non-local overlay must replace it with External Secrets,
Sealed Secrets, or another cluster secret provider. It must also replace the
single-node PostgreSQL, Kafka, and Redis manifests with managed services or
operator-owned highly available clusters. Three application pods do not make
those stateful dependencies highly available.

Before treating this as production-ready, use immutable registry tags, enable
TLS and authentication for gRPC/Kafka/Redis/PostgreSQL, configure backups and
restore drills, and change the NetworkPolicy public ingress rule to select the
actual ingress-controller namespace instead of allowing all source IPs.
