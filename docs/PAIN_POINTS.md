# ZSGate Pain Points and Product Mapping

This document tracks core user pain points and corresponding OSS/Enterprise capabilities.

## 1) Company token transparency
Pain:
- Managers cannot clearly answer who used company tokens and for what.

Mapped capabilities:
- OSS: `/admin/usage/by-user`, `/admin/costs/by-user`, `/admin/audits`
- Enterprise: deeper compliance reports, approval-linked responsibility chain

## 2) Task classification and workload visibility
Pain:
- Hard to understand usage by business scenario (coding/analysis/ops/translation).

Mapped capabilities:
- OSS: auto task classification (`task_category`), `/admin/usage/by-scenario`
- Enterprise: policy-driven classification packs and governance dashboards

## 3) Real-time active usage
Pain:
- Need to know which employees are active right now and what tasks are running.

Mapped capabilities:
- OSS: `/admin/realtime/active-users`, `/admin/realtime/active-tasks`, `/admin/realtime/stream`
- Enterprise: multi-cluster real-time board and incident linkage

## 4) Real-time conversation flow
Pain:
- Need live conversation event stream for operations and governance.

Mapped capabilities:
- OSS: `/admin/realtime/dialog-stream` (sanitized summaries by default)
- Enterprise: full retention playback and compliance-grade access control

## 5) Alerting
Pain:
- Teams need timely alerts for abnormal cost/error/latency spikes.

Mapped capabilities:
- OSS: threshold alerts (`error_rate`, `latency_p95`, `cost_spike`) with optional webhook
- Enterprise: advanced alert routing, suppression, escalation, SLO burn-rate
