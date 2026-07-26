---
name: trackfw-iac-skill
description: Declarative infrastructure provisioning, state management, policy-as-code and immutability.
---

## Foundational Principles

Infrastructure as Code means the repository is the authoritative source of truth for every
provisioned resource. If it is not in code, it does not officially exist.

- **Declarative over imperative**: describe the desired end state; let the tool reconcile.
- **Idempotent by design**: applying the same configuration twice must produce the same result.
- **Immutable infrastructure**: replace, do not patch. Mutable drift between nodes is a source
  of undocumented operational risk.
- **Least privilege**: every resource receives the minimum permissions required, nothing more.
  Wildcard actions combined with wildcard resources are forbidden.

## File and Module Structure

Organize files by concern, not by resource count. In Terraform/OpenTofu:

```
infra/<environment>/
├── main.tf       # primary resources
├── variables.tf  # typed, described, validated inputs
├── outputs.tf    # values consumed by other configurations
├── versions.tf   # required_version and required_providers (pinned)
├── locals.tf     # computed values, naming conventions, shared tags
└── data.tf       # data sources, separate from managed resources
```

Never put everything in `main.tf`. One file per concern makes diffs reviewable and
`terraform plan` output readable.

Module decisions:
- Create a local module when a pattern repeats three or more times in the same project.
- Prefer a well-maintained registry module over a custom one for complex resources.
- Always pin module versions: `version = "~> 4.0"`. Unpinned modules are a supply-chain risk.

## State Management

Remote state is mandatory for team and production environments:
- Store in a managed backend with encryption at rest and distributed locking.
- Never commit `terraform.tfstate` or `*.tfvars` containing secrets.
- Use workspaces or separate state files per environment; mixing environments in a single
  state file is an operational hazard.
- Enable state file backup and versioning on the storage backend.

## Plan, Review and Apply

```
1. terraform fmt -check -recursive   # style
2. terraform validate                 # syntax and provider schema
3. policy-as-code scan (Checkov, Trivy config)  # security gates
4. cost estimation (Infracost)        # visibility before apply
5. terraform plan -out=plan.tfplan    # deterministic plan
6. human review of plan               # mandatory for production
7. terraform apply plan.tfplan        # apply the reviewed plan
```

**Never apply to production without a reviewed, saved plan.** Ad-hoc `terraform apply` in
production bypasses the review gate and defeats reproducibility.

## Policy-as-Code

Automated policy checks run before `apply`, not after:
- Reject resources with encryption disabled (storage, databases, secrets).
- Reject IAM policies with `"*"` on both action and resource simultaneously.
- Require mandatory tags (environment, team, cost-centre) on all taggable resources.
- Enforce minimum TLS versions and private endpoint requirements.
- HIGH and CRITICAL policy violations block the pipeline; they are not warnings.

## Configuration Management (Ansible)

- Every task must be idempotent: running the playbook twice must not change the system on the
  second run.
- Use `no_log: true` on every task that handles secrets or credentials.
- Store secrets in a secrets manager or vault; never in playbook variables or inventory files.
- Use dynamic inventory sources; never hardcode hostnames or IP addresses.
- Name every task descriptively; unnamed tasks make failure logs unreadable.

## Anti-Patterns

The following are always wrong:
- `apply` in production without a human-reviewed plan.
- Secrets in `.tfvars`, environment variables without a secrets manager, or hardcoded values.
- `"*"` in both `actions` and `resources` of an IAM statement.
- Unpinned module or provider versions.
- `null_resource` or `local-exec` for business logic that a provider resource can handle.
- Manual changes to resources managed by IaC (infrastructure drift).
