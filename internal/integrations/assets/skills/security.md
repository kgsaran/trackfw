---
name: trackfw-security-skill
description: Threat modeling, DevSecOps controls, Zero Trust and supply chain security.
---

## Shift-Left Security

Security controls belong in the development pipeline, not only at the perimeter. The later a
vulnerability is found, the more expensive it is to fix.

CI pipeline mandatory gates:
- **SAST** (Static Application Security Testing): scan source code on every PR.
- **SCA** (Software Composition Analysis): identify vulnerable dependencies; block on HIGH/CRITICAL.
- **Secrets scanning**: detect committed credentials before they reach the repository.
- **Container image scanning**: scan base images and final images; fail on HIGH/CRITICAL CVEs.
- **IaC security**: scan Terraform, Helm and Kubernetes manifests for misconfigurations.

## Threat Modeling

Model threats before writing code for any new feature that handles authentication, authorisation,
data storage or external integrations. Use STRIDE as the baseline framework:

| Threat | Example |
|---|---|
| Spoofing | Forged identity or token |
| Tampering | Modified request payload |
| Repudiation | Missing audit trail |
| Information Disclosure | Over-permissive API response |
| Denial of Service | Unbounded resource consumption |
| Elevation of Privilege | Broken access control |

Validate findings against OWASP Top 10 and ASVS (Application Security Verification Standard)
at the appropriate assurance level for the feature.

## Zero Trust Architecture

- **Never trust, always verify**: authenticate and authorize every request, regardless of network
  origin. Internal services are not implicitly trusted.
- **mTLS** between services in a mesh: mutual authentication proves both sides hold valid certificates.
- **Least privilege at every layer**: IAM roles, Kubernetes RBAC, database users — minimum
  permissions for the task, nothing more.
- **JIT (Just-in-Time) access**: elevate privilege for a bounded time window for administrative
  tasks; revoke automatically.

## Identity and Access Management

- OAuth 2.0 with PKCE for public clients; validate `iss`, `aud` and `exp` on every JWT.
- RBAC for coarse-grained access; ABAC for attribute-based decisions.
- Rotate credentials on schedule and on any suspicion of compromise.
- Secrets management: use a dedicated secrets manager; never hardcode secrets in code,
  configuration files or environment variables committed to a repository.

## Supply Chain Security

- Generate an SBOM (Software Bill of Materials) in CycloneDX or SPDX format on every build.
- Sign artifacts and container images (Sigstore/Cosign); verify signatures in admission
  controllers before deploying to production.
- Pin dependency versions; review updates in PRs, not silently via floating ranges.
- Monitor for newly published CVEs against your SBOM continuously, not just at build time.

## API Security

Design APIs with OWASP API Security Top 10 in mind:
- Broken Object Level Authorization (BOLA) is the most common API vulnerability — validate
  that the caller has access to the specific object, not just the endpoint.
- Rate-limiting and input validation are required on every public endpoint.
- Avoid exposing internal IDs directly; prefer opaque tokens or UUIDs.

## Incident Readiness

- Centralise logs and security events in a SIEM; define alerting thresholds before incidents occur.
- Maintain and rehearse an incident response playbook; detection-to-containment time (MTTC)
  is the metric that matters.
- Conduct a post-incident review (PIR) after every significant security event; update controls.
