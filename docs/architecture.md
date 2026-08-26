# CLEP Architecture

## System Overview

CLEP (Cloud Native Learning Engineering Platform) is a self-hosted learning environment that transforms CNPE preparation from passive reading into active engineering through AI-driven projects, labs, and Socratic tutoring.

## Architecture Diagram

```
                    INTERNET
                        │
                        ▼
                 ┌──────────────┐
                 │  Reverse     │
                 │  Proxy       │
                 └──────┬───────┘
                        │
          ┌─────────────┼─────────────┐
          │             │             │
          ▼             ▼             ▼
       Plane        CLEP API      Grafana
          │             │
          │             ▼
          │       Agent Gateway
          │             │
          │       ┌─────┴─────┐
          │       │           │
          │       ▼           ▼
          │    Event Bus   Context
          │                 Builder
          │
          ▼
     Plane Webhooks

External:
────────────────────────────────────

GitHub ──────────────┐
                     │
Notion ──────────────┼──► Integration
                     │
CNCF/Kubernetes ─────┘

Internal:
────────────────────────────────────

Agent Gateway
      │
      ├── Hermes
      ├── PostgreSQL
      ├── Redis
      ├── Qdrant
      ├── MinIO
      └── Kubernetes Labs
```

## Component Responsibilities

### Plane
- Human-facing control plane
- Work items, projects, cycles
- Webhooks for event generation
- Issue comments for interaction

### CLEP Gateway
- Webhook ingestion and validation
- Event normalization
- NATS pub/sub
- Context building
- Session management

### Hermes Adapter
- Consumes NATS events
- Calls Hermes Agent
- Streams responses back
- Updates Plane comments

### PostgreSQL
- Event persistence
- Agent session state
- Learning progress
- Knowledge metadata

### NATS JetStream
- Durable event streams
- Decoupling services
- Retry and replay
- Pub/sub messaging

## Event Flow

```
Plane Comment (@hermes)
    ↓
Webhook to Gateway
    ↓
HMAC Validation
    ↓
Event Normalization
    ↓
NATS Publish (clep.events)
    ↓
Hermes Adapter Subscribe
    ↓
Context Build (Plane + DB)
    ↓
Hermes Query (streaming)
    ↓
Response Stream to Plane
    ↓
Comment Updated Incrementally
```

## Database Schema

```sql
-- Events
events (id, event_type, source, payload, created_at, status)

-- Agent Sessions
agent_sessions (id, event_id, work_item_id, workspace_id, status, created_at, completed_at)

-- Agent Messages
agent_messages (id, session_id, role, content, created_at)

-- Plane Comments (cached)
plane_comments (id, comment, work_item_id, workspace_id, project_id, created_by, created_at)
```

## Security

- HMAC-SHA256 webhook validation
- Kubernetes Secrets for credentials
- Network policies for isolation
- Least privilege service accounts
- Audit logging

## Deployment

### Minimal (Local)
```bash
kubectl apply -f deploy/k8s/nats.yaml
kubectl apply -f deploy/k8s/postgres.yaml
kubectl apply -f deploy/k8s/gateway.yaml
kubectl apply -f deploy/k8s/hermes-adapter.yaml
```

### GitOps (Flux CD)
```bash
flux bootstrap github --owner=EzzOps --repository=clep --branch=main
```

## Future Phases

### Phase 2 - Knowledge Platform
- CNPE curriculum ingestion
- Qdrant vector store
- Embedding generation
- RAG retrieval

### Phase 3 - Engineering Projects
- GitHub integration
- PR review automation
- Project rubric generation

### Phase 4 - Kubernetes Labs
- Lab controller
- Failure injection
- Automated testing

### Phase 5 - Autonomous Learning
- Competency engine
- Adaptive project generation
- Exam simulation
