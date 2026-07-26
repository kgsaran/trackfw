---
name: trackfw-architecture-skill
description: System design, architectural patterns, trade-off analysis and cross-cutting decisions.
---

## Architectural Thinking

Prefer explicit trade-off analysis over pattern application. Every architectural decision carries
costs — document them in an ADR before coding. Use the C4 model (Context → Container →
Component → Code) to make boundaries visible and reviewable.

## Structural Patterns

**Layered / Clean Architecture** — enforce one-way dependency: domain has no infrastructure
imports. Validate layer boundaries with architecture fitness functions (ArchUnit, go-arch-lint).

**Hexagonal (Ports & Adapters)** — inbound ports (use cases) are driven by the application;
outbound ports (repositories, messaging, external APIs) are driven by infrastructure. Swap
adapters without touching domain logic.

**Domain-Driven Design (DDD)** — identify bounded contexts before assigning microservices. Use
a ubiquitous language inside each context; translate explicitly at anti-corruption layers.

**Microservices** — decompose by business capability, not by technical tier. Each service owns
its data store; inter-service communication via API contracts (OpenAPI, Protobuf) or events.

## Event-Driven Architecture

- Prefer events over direct calls for cross-context side effects.
- Use the Outbox pattern to guarantee at-least-once delivery without distributed transactions.
- Apply CQRS to separate read models (optimized projections) from write models (command handlers).
- Event Sourcing: store state as an immutable log of events; derive projections on read.
- Saga pattern (choreography or orchestration) for multi-step distributed transactions.

## Cloud & Distributed Systems

- CAP theorem: partition tolerance is mandatory in distributed systems — choose consistency vs
  availability per use case, not globally.
- ACID vs BASE: relational transactions for strong consistency; eventual consistency acceptable
  for high-availability reads with idempotent writes.
- Well-Architected pillars: reliability, security, performance efficiency, cost optimization,
  operational excellence, sustainability.
- Prefer stateless services; externalize session, cache and state.
- 12-Factor App: config from environment, logs to stdout, disposable processes, dev/prod parity.

## AI / LLM Architecture

- RAG pipelines: design chunking strategy before choosing an embedding model. Hybrid retrieval
  (sparse + dense with Reciprocal Rank Fusion) outperforms dense-only in most production workloads.
- Multi-agent systems: declare orchestration topology (sequential / parallel / hierarchical)
  and tool-calling boundaries in the design phase. Human-in-the-loop for irreversible actions.
- LLM observability: trace prompts, token counts and latency; monitor for drift and quality
  degradation using structured evaluation datasets.

## Platform Engineering

- DORA metrics (Deployment Frequency, Lead Time, MTTR, Change Failure Rate) are lagging
  indicators — optimize for them by improving deployment pipeline and incident detection.
- Internal Developer Platforms reduce cognitive load via golden paths and self-service scaffolding.
- FinOps: tie cost allocation to teams and services; make spend visible before optimizing it.

## Decision Records

Write an ADR for every significant architectural decision: the context, the options evaluated,
the chosen option and its consequences. A decision without documented trade-offs cannot be
revisited or reversed safely.
