# CNPE Learning Engineering Platform — CLEP

## 1. Product Vision

Transform CNPE preparation from:

```
read → watch → memorize → quiz
```

into:

```
curriculum → diagnose → project → build → break → troubleshoot → explain → review → exam → measure → next project
```

## 2. Core Principles

### 2.1 Curriculum-Driven
Every learning activity maps to:

```
CNPE
  ↓
Domain
  ↓
Competency
  ↓
Knowledge concept
  ↓
Practical capability
  ↓
Project
  ↓
Assessment
```

### 2.2 Project-First
Hermes generates projects rather than explanations.

### 2.3 Active Learning
Socratic questioning:

```
question → user attempt → hint → deeper question → explanation
```

### 2.4 Event-Driven
Never manually ask Hermes what to do.

### 2.5 Persistent Context
Hermes remembers what you've built, failed, misunderstood.

### 2.6 Real Engineering
Every competency produces:

```
code
IaC
Kubernetes manifests
architecture
tests
incident reports
design decisions
```

### 2.7 Evidence-Based
Sources cited, hallucinations prevented.

## 3. Technology Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| Control Plane | Plane | Work items, projects, comments |
| Event Bus | NATS JetStream | Durable messaging |
| Database | PostgreSQL | State persistence |
| Vector Store | Qdrant | Knowledge retrieval |
| Object Storage | MinIO | Artifacts |
| Agent | Hermes | Learning intelligence |
| CI/CD | Flux CD | GitOps deployment |
| Monitoring | Prometheus + Grafana | Observability |

## 4. MVP Scope

### MVP #1: Ask Hermes from Plane

**Goal:** Prove the core interaction loop

**Flow:**
```
Plane comment (@hermes) → Webhook → Gateway → NATS → Hermes → Stream → Plane comment
```

**Success Criteria:**
1. Create Plane issue: CNPE-NET-001
2. Add comment with `@hermes`
3. Webhook fires to Gateway
4. Gateway validates and creates event
5. Hermes responds
6. Comment streams incrementally
7. Status tracked in database

**NOT in MVP:**
- CNPE RAG
- Notion sync
- GitHub integration
- Kubernetes labs
- Competency scoring
- Automatic project generation
- Exam simulation

## 5. Repository Structure

```
clep/
├── gateway/              # FastAPI webhook service
│   ├── main.go
│   ├── database.go
│   ├── plane_client.go
│   ├── hermes_client.go
│   └── events.go
├── contracts/            # Shared schemas
│   └── events.go
├── deploy/
│   ├── k8s/
│   │   ├── nats.yaml
│   │   ├── postgres.yaml
│   │   ├── gateway.yaml
│   │   └── hermes-adapter.yaml
│   ├── helm/
│   │   └── gateway/
│   └── flux/
│       └── clep.yaml
├── docs/
│   ├── architecture.md
│   └── PRD.md
├── .github/
│   └── workflows/
│       ├── ci.yaml
│       └── deploy.yaml
├── Dockerfile
├── go.mod
├── README.md
└── .gitignore
```

## 6. Deployment

### Prerequisites
- Kubernetes cluster (k3s, EKS, GKE, or kind)
- Helm 3
- kubectl configured
- GitHub PAT for Flux
- Plane instance with API access

### Quick Start
```bash
# Create namespace
kubectl create namespace clep

# Deploy infrastructure
kubectl apply -f deploy/k8s/nats.yaml -n clep
kubectl apply -f deploy/k8s/postgres.yaml -n clep

# Configure secrets
kubectl apply -f deploy/k8s/secrets.yaml -n clep

# Deploy services
kubectl apply -f deploy/k8s/gateway.yaml -n clep
kubectl apply -f deploy/k8s/hermes-adapter.yaml -n clep

# Verify
kubectl get pods -n clep
kubectl logs -f deploy/clep-gateway -n clep
```

### Plane Configuration
1. Create workspace: `CNPE Academy`
2. Create project: `01 Platform Foundations`
3. Add webhook: `https://clep.your-domain.com/webhooks/plane`
4. Select events: `issue.comment.create`
5. Configure secret in K8s: `PLANE_WEBHOOK_SECRET`

## 7. Event Schema

```json
{
  "id": "evt_1234567890",
  "event_type": "plane.comment.created",
  "source": "plane",
  "payload": {
    "comment": {
      "id": "abc123",
      "comment": "@hermes explain why kube-proxy is needed",
      "work_item_id": "WORK-123",
      "workspace_id": "ws-123",
      "project_id": "proj-123",
      "created_by": "user-123"
    },
    "work_item": {
      "id": "WORK-123",
      "identifier": "CNPE-NET-001",
      "title": "Kubernetes Networking",
      "description": "..."
    },
    "previous_comments": []
  },
  "timestamp": "2026-08-26T10:00:00Z",
  "status": "created"
}
```

## 8. API Endpoints

### Gateway
- `POST /webhooks/plane` - Receive Plane webhooks
- `GET /health` - Health check
- `GET /events/:id` - Event status

### Hermes Adapter
- `GET /health` - Health check

## 9. Security

- HMAC-SHA256 webhook validation
- Kubernetes Secrets for credentials
- Network policies for isolation
- Principle of least privilege
- Audit logging

## 10. Roadmap

| Phase | Features | Timeline |
|-------|----------|----------|
| MVP #1 | Plane ↔ Hermes loop | Week 1 |
| MVP #2 | Knowledge platform (Qdrant) | Week 2-3 |
| MVP #3 | GitHub integration | Week 4 |
| Phase 2 | Kubernetes labs | Week 5-6 |
| Phase 3 | Competency engine | Week 7-8 |
| Phase 4 | Exam simulation | Week 9-10 |

## 11. License

MIT
