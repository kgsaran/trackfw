---
name: trackfw-infra-skill
description: Cloud platforms, Kubernetes operations, CI/CD, observability and reliability.
---

## Kubernetes Operations

- GitOps is the source of truth: cluster state is declared in a repository and reconciled
  continuously by a controller (ArgoCD, Flux). Manual `kubectl apply` in production is an
  anti-pattern.
- Use readiness and liveness probes on every workload; tune them to reflect actual service
  health, not just process health.
- Resource requests and limits are mandatory; workloads without them destabilize the
  node's QoS. Rightsize with Vertical Pod Autoscaler metrics, not guesses.
- RBAC: least privilege per ServiceAccount. Never use the `cluster-admin` role for workloads.
- Pod Security Standards (restricted profile) as the default; relax per workload only with
  documented justification.

## CI/CD

- Pipelines are code: version-controlled, reviewable, tested.
- Use environment-specific deployment gates; production deployments require explicit approval.
- Separate build from deploy: the same artifact that passed staging is promoted to production
  (immutable artifact principle).
- Reusable workflows reduce duplication; extract common steps (lint, test, scan, notify).
- Dependency scanning and container image scanning run on every PR, not only on schedule.

## Observability

Instrument with OpenTelemetry as the vendor-neutral standard:
- **Metrics**: RED method (Rate, Errors, Duration) for services; USE method (Utilization,
  Saturation, Errors) for resources.
- **Traces**: distributed trace context propagates across all service calls; sample based on
  latency or error, not uniformly.
- **Logs**: structured JSON to stdout; correlate with trace ID.
- Define SLOs and error budgets before an incident, not during.
- Golden signals dashboard is the first screen in any incident response.

## Reliability

- Chaos engineering (controlled failure injection) validates reliability assumptions before
  production incidents do. Start with single-node failures, then dependency outages.
- Disaster recovery: define RTO and RPO per service tier; test backup restoration on schedule
  — untested backups are not backups.
- Multi-AZ for stateful workloads; cross-region only where business continuity justifies cost.
- Circuit breakers at the service mesh layer protect downstream services from cascading failures.

## Cloud Cost Optimization (FinOps)

- Tag every resource with team, environment and cost centre at provisioning time; retroactive
  tagging is never complete.
- Spot/preemptible instances for stateless, fault-tolerant workloads; reserved capacity for
  predictable baseline load.
- Rightsizing is a continuous process; review monthly, not quarterly.
- Cost visibility per team is a prerequisite for accountability.

## Supply Chain Security

- Sign container images with Sigstore/Cosign; verify signatures in admission controllers.
- Generate SBOM (CycloneDX or SPDX) on every build; store alongside the artifact.
- Pin base image digests, not tags; floating tags introduce silent updates.
- Scan dependencies for known CVEs at build time and on a regular schedule.
