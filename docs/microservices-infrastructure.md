# Microservices infrastructure

This directory is the deployment scaffold for the strangler migration. The
public RealWorld contract remains owned by `gateway`; all other applications
are internal gRPC services.

| Application | Port | Database | Binary |
| --- | ---: | --- | --- |
| gateway | 8000 HTTP | `conduit_gateway` | `/app/conduit` |
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

The existing root `docker-compose.yml` is unchanged and remains the supported
monolith workflow. The microservice scaffold is separate:

```bash
docker-compose -f deploy/compose/docker-compose.microservices.yml up -d postgres kafka redis gateway-migrate gateway
```

This starts the current gateway against its own database and brings up the
platform dependencies. After all service binaries have been added to the
image, enable the complete topology:

```bash
docker-compose -f deploy/compose/docker-compose.microservices.yml --profile microservices up --build
```

The development credentials in the Compose file are intentionally local-only.
The PostgreSQL bootstrap scripts run only when its volume is first created. If
the database list changes, create the missing database manually; deleting the
volume destroys all local data and should only be done deliberately.

Useful host endpoints are gateway `localhost:8000`, Kafka
`localhost:29092`, Redis `localhost:6379`, and PostgreSQL `localhost:5432`.
Service ports `9001` through `9005` are published for `grpcurl` and debugging.

## Kubernetes

`deploy/kubernetes/base` is a Kustomize base suitable for kind or k3d. It
contains:

- three replicas for every stateless application;
- a public `LoadBalancer` Service only for gateway;
- native HTTP/gRPC startup, readiness, and liveness probes;
- PodDisruptionBudgets, HPAs, rolling updates, and topology spreading;
- default-deny NetworkPolicies with explicit application, platform, and DNS
  traffic;
- one development PostgreSQL, Kafka KRaft broker, and Redis instance.

The development base uses one `conduit:local` image containing multiple
binaries. A production overlay can replace it with independently versioned
service images. Build or load the local image, then render and apply:

```bash
kubectl kustomize deploy/kubernetes/base
kubectl apply -k deploy/kubernetes/base
kubectl apply -k deploy/kubernetes/jobs
kubectl -n conduit get pods,svc,hpa,pdb
```

The gateway migration job uses the current `/app/migrations` directory. Jobs
for extracted services are created suspended because their migration trees do
not exist yet. Once a service image owns its migrations, delete and recreate
the job if necessary, then unsuspend it, for example:

```bash
kubectl -n conduit patch job subscriptions-migrate --type merge -p '{"spec":{"suspend":false}}'
```

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
