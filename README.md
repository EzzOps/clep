# CLEP - Cloud Native Learning Engineering Platform

A self-hosted learning platform that transforms CNPE preparation from passive reading into active engineering through AI-driven projects, labs, and Socratic tutoring.

## Architecture

```
                    YOU
                     │
                     ▼
          ┌─────────────────────┐
          │       PLANE         │
          │   Learning Cockpit  │
          │  Projects & Tasks   │
          └──────────┬──────────┘
                     │
                Webhooks
                     │
                     ▼
          ┌─────────────────────┐
          │   CLEP GATEWAY      │
          │  Event Router       │
          │  Context Builder    │
          └───────┬─────────────┘
                  │
         ┌────────┼──────────┐
         │        │          │
         ▼        ▼          ▼
      Hermes   Knowledge   Learning
      Agent     Platform    Engine
         │        │          │
         └────────┼──────────┘
                  │
           Evidence + State
```

## MVP #1: Ask Hermes from Plane

The first vertical slice proves the core interaction loop:

```
You comment "@hermes explain why kube-proxy is needed"
    ↓
Plane webhook → Gateway → NATS
    ↓
Hermes adapter processes event
    ↓
Streams response back to Plane comment
```

## Components

- **gateway/** - FastAPI service handling webhooks, NATS pub/sub, Plane API integration
- **contracts/** - Shared event schemas and types
- **deploy/** - Kubernetes manifests, Helm charts, Flux CD configuration
- **docs/** - Documentation and architecture guides

## Getting Started

### Prerequisites

- Kubernetes cluster (k3s, EKS, GKE, or kind)
- Helm 3
- kubectl configured
- GitHub PAT for Flux CD
- Plane instance with API access

### Quick Deploy (Local Testing)

```bash
# Create namespace
kubectl create namespace clep

# Deploy NATS
kubectl apply -f deploy/k8s/nats.yaml -n clep

# Deploy PostgreSQL
kubectl apply -f deploy/k8s/postgres.yaml -n clep

# Configure secrets
kubectl apply -f deploy/k8s/secrets.yaml -n clep

# Deploy Gateway
kubectl apply -f deploy/k8s/gateway.yaml -n clep

# Deploy Hermes Adapter
kubectl apply -f deploy/k8s/hermes-adapter.yaml -n clep
```

### GitOps Deploy (Flux CD)

```bash
# Install Flux
flux bootstrap github \
  --owner=EzzOps \
  --repository=clep \
  --branch=main \
  --path=./deploy/flux \
  --personal

# Apply credentials
kubectl apply -f deploy/flux/credentials.yaml -n flux-system

# Flux will auto-sync from repo
```

### Plane Configuration

1. Create workspace: `CNPE Academy`
2. Create project: `01 Platform Foundations`
3. Add webhook: `https://clep.your-domain.com/webhooks/plane`
4. Select events: `issue.comment.create`
5. Configure signature: Generate secret and set `PLANE_WEBHOOK_SECRET`

## Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `PORT` | Gateway port (default: 8080) | No |
| `NATS_URL` | NATS server URL | Yes |
| `PLANE_API_URL` | Plane API endpoint | Yes |
| `PLANE_API_KEY` | Plane API token | Yes |
| `PLANE_WEBHOOK_SECRET` | Webhook HMAC secret | Yes |
| `DATABASE_URL` | PostgreSQL connection string | Yes |
| `HERMES_ADAPTER_URL` | Hermes adapter service URL | Yes |

## Event Flow

1. User comments in Plane with `@hermes`
2. Plane sends webhook to Gateway
3. Gateway validates HMAC signature
4. Gateway normalizes event and publishes to NATS
5. Hermes adapter subscribes to NATS
6. Adapter builds context from Plane + database
7. Adapter calls Hermes with streaming
8. Response streamed back to Plane comment
9. Event status updated to `completed`

## API Endpoints

### Gateway

- `POST /webhooks/plane` - Receive Plane webhooks
- `GET /health` - Health check
- `GET /events/:id` - Event status

### Hermes Adapter

- `GET /health` - Health check

## Database Schema

```sql
-- Events table
CREATE TABLE events (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL,
  source TEXT NOT NULL,
  payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL
);

-- Agent sessions
CREATE TABLE agent_sessions (
  id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL,
  work_item_id TEXT NOT NULL,
  workspace_id TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

-- Agent messages
CREATE TABLE agent_messages (
  id SERIAL PRIMARY KEY,
  session_id TEXT NOT NULL,
  role TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);
```

## Security

- All webhooks validated with HMAC-SHA256
- Secrets stored in Kubernetes Secrets
- Network policies restrict inter-service communication
- Principle of least privilege for all service accounts

## Next Steps

After MVP #1 is working:

1. Add CNPE curriculum ingestion
2. Build knowledge platform (Qdrant + embeddings)
3. Add GitHub integration for PR reviews
4. Implement Kubernetes lab controller
5. Add competency scoring engine
6. Build exam simulation module

## License

MIT
