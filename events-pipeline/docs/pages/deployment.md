Deployment of Event Pipeline
---

### Pre-requisites

- A Kafka server, which must be located within the same data center to maintain low latency.
- For production/staging, Transport Layer Security (TLS) must be enabled. The necessary secrets are fetched during the deployment phase.
- Doppler (event-pipeline) secret is required for managing environment variables.

## Deployment Dockerfiles

- Rust Capture Deployment: Refer to `rust-capture/Dockerfile.`
- Kafka Streams Deployment: Refer to `kafka-streams/Dockerfile.`

Deployment files are under the `app-manifests-prod` repository.
