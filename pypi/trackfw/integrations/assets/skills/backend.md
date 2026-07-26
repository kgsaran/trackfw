---
name: trackfw-backend-skill
description: Backend APIs, domain logic, persistence patterns and service reliability.
---

## Architecture

Follow Clean Architecture with a strict one-way dependency rule:
Handler → Service → Repository → External. Domain and service layers must not import
infrastructure packages. Enforce layer boundaries with architecture fitness functions.

Use dependency injection to wire components; keep constructors free of business logic.
One handler = one responsibility. Prefer small, focused interfaces (ISP) over fat interfaces.

## REST APIs

- HTTP status codes must be semantically correct; use 4xx for client faults, 5xx for server
  faults. Never return 200 with an error body.
- Error responses follow RFC 7807 `application/problem+json`: `type`, `title`, `status`,
  `detail`, optional `instance`.
- Pagination: cursor-based for large or frequently updated datasets; offset-based for stable
  datasets. Expose `page`, `limit`, `total` and a `next` link where applicable.
- Version contracts in the URL path (`/v1/`) or via `Accept` header; deprecate, do not remove.
- Document with OpenAPI 3.1; generate from code or validate generated code against the spec.

## gRPC / Protobuf

- Define contracts in `.proto` files; version the package name (`v1`, `v2`).
- Use buf CLI for linting, breaking-change detection and code generation.
- gRPC-Gateway for HTTP/JSON bridge when REST clients must be supported alongside gRPC.
- Unary RPCs for request-response; server-streaming for long polls; bidirectional streaming
  only when both sides need to send at their own pace.

## Reliability Patterns

- **Circuit Breaker**: open after N consecutive failures; half-open to probe recovery.
- **Retry with exponential backoff + jitter**: never retry non-idempotent mutations without
  an idempotency key.
- **Idempotency keys**: required for any state-changing operation exposed over HTTP or messaging.
- **Saga / Outbox**: use Outbox to publish events transactionally; use Saga for multi-service
  workflows where distributed transactions are not available.
- **CQRS**: separate command models (write, validated, consistent) from query models
  (read-optimized projections); avoid mixing concerns in the same handler.

## Observability

Instrument every service with structured logs, metrics and distributed traces (OpenTelemetry).
Log at the boundary (incoming request, outgoing call, error); avoid logging inside loops.
Trace context must propagate across all inter-service calls.

## Data Access

Repository pattern: one repository per aggregate root. The repository interface belongs to the
domain; the implementation belongs to infrastructure. Never leak query language or driver types
into the service layer.

Cache-aside for read-heavy data: read from cache first, populate on miss, invalidate on write.
Write-through when consistency between cache and store is critical.

Wrap every database error: `fmt.Errorf("repo.FindByID: %w", err)` (Go) or a typed domain
exception (Java). Callers must not receive raw driver errors.

## Testing

Prefer integration tests that hit a real database over mocked repositories; mock divergence
from production behaviour is a common source of post-deployment failures.
Unit-test domain/service logic in isolation. Aim for 85%+ coverage on the service layer.
