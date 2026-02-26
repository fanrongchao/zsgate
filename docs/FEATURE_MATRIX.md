# ZSGate Feature Matrix (OSS vs Enterprise)

## Positioning
- OSS: solve team-level common pain points and maximize adoption.
- Enterprise: solve org-level governance, compliance, and scale operations.

## Capability Matrix

| Capability | OSS (zsgate) | Enterprise (zsgate-enterprise) |
|---|---|---|
| Multi-vendor gateway (OpenAI/Claude/Gemini/vLLM) | Yes | Yes |
| OpenAI-compatible APIs | Yes | Yes |
| Model alias + weighted routing + failover | Yes | Yes |
| Basic vendor management console APIs | Yes | Yes |
| Basic usage metrics and cost estimate | Yes | Yes |
| Team analytics by user/project/department | Yes | Yes |
| Basic audit trail and trace search | Yes | Yes |
| MCP integration (basic) | Yes | Yes |
| Single-cluster private deployment | Yes | Yes |
| OIDC/SSO enterprise connectors | No | Yes |
| Advanced RBAC and permission packs | No | Yes |
| Approval workflows and policy engine | No | Yes |
| Compliance report templates and governance exports | No | Yes |
| Multi-cluster / multi-region control operations | No | Yes |
| Enterprise security integrations (KMS/HSM policy packs) | No | Yes |
| SLA, support tooling, and enterprise operations package | No | Yes |

## Product Boundary Rules
1. Do not move baseline usability features from OSS to Enterprise.
2. Keep OSS fully usable for real internal team usage.
3. Keep Enterprise value in governance depth, compliance depth, and scale operations.
4. When adding new features, classify by "team common need" (OSS) vs "org governance need" (Enterprise).

## Why This Boundary Works
- OSS remains attractive and practical, which drives adoption and brand growth.
- Enterprise retains clear budget-justified value for CIO/Security/Compliance buyers.
- Engineering can evolve both editions in parallel without product confusion.
