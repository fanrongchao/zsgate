# ZSGate OSS / Enterprise Boundary

## Repositories
- OSS repo: `zsgate`
- Private repo: `zsgate-enterprise`

## What stays in OSS
- Core API gateway
- Multi-provider adapters
- Basic routing/fallback
- Basic usage and audit events
- Community deployment manifests

## What stays in Enterprise
- SSO/OIDC connectors and enterprise IAM integrations
- Advanced RBAC and approval workflows
- Compliance reports and policy packs
- Multi-cluster/multi-region control operations
- Enterprise support tooling and operational dashboards

## Integration Principle
- Keep stable northbound APIs in OSS.
- Add enterprise-only administration APIs in private repo.
- Avoid changing OSS internals directly for enterprise features.
