# Eventing and transactional outbox

Domain commands remain synchronous. Follow, unfollow, favorite, unfavorite,
article deletion, and profile update return only after their PostgreSQL mutation
commits. Database triggers add an outbox row in the same transaction; they do
not publish directly to Kafka.

Three relay processes claim committed rows with `FOR UPDATE SKIP LOCKED`,
publish with franz-go's idempotent producer and all-ISR acknowledgements, then
mark the row published. Failed rows are unlocked with capped exponential
backoff. Two relay replicas may safely process one database.

| Topic | Event types |
| --- | --- |
| `conduit.subscriptions` | `subscription.created.v1`, `subscription.deleted.v1` |
| `conduit.articles` | `article.favorited.v1`, `article.unfavorited.v1`, `article.deleted.v1` |
| `conduit.profiles` | `profile.updated.v1` |

Every message contains `event_id`, `event_type`, `event_version`, `source`,
`aggregate_id`, `occurred_at`, optional `correlation_id`, and typed JSON data.
The aggregate ID is the Kafka key, preserving order for one aggregate.

`InboxStore` provides lease-based claim, complete, and release primitives for
idempotent consumers. A handler that also changes PostgreSQL data should call
the equivalent inbox queries in the same local transaction as its changes.
Kafka offsets are committed only after inbox completion. Delivery remains
at-least-once, so handlers must be idempotent.

Local Compose starts one relay per event-producing database in the
`microservices` profile after an idempotent init container has created all
three topics. Metrics are exposed on host ports 9101 through 9103.
Kubernetes runs two replicas per relay with topology spreading and a
PodDisruptionBudget. The metrics use only fixed `operation` and `result` label
values; event IDs, keys, errors, and payloads never become labels.

Development Kafka is plaintext. Production configuration must set
`KAFKA_TLS=true` and, when SASL/PLAIN is appropriate, `KAFKA_USERNAME` and
`KAFKA_PASSWORD`. Credentials belong in a Secret rather than the shared
ConfigMap.
