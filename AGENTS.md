# AGENTS.md - ZSGate OSS

## 1. 仓库目标与非目标
目标：维护可直接被团队使用的开源 LLM 网关能力（多 vendor、基础审计、统计、实时可见、基础告警）。
非目标：企业专属治理深度能力（审批流引擎、企业 SSO 深度集成、合规取证全流程、多集群控制）。

## 2. 目录职责
- `control-plane/`: 管理与统计、审计、告警、实时 API。
- `data-plane/`: 鉴权、路由、转发、事件上报。
- `pkg/common/`: control/data 共享类型。
- `docs/`: 对外文档，必须与代码能力一致。

## 3. 开发流程
1. 先阅读：`README.md`、`docs/OSS_ENTERPRISE_BOUNDARY.md`、`docs/FEATURE_MATRIX.md`。
2. 涉及接口改动时先确认是否影响 `/v1/*`、`/admin/*` 的兼容性。
3. 修改代码后至少执行：
   - `GOCACHE=/tmp/go-build go build ./control-plane/cmd/server ./data-plane/cmd/server`
4. 涉及能力变更时同步更新文档。

## 4. 兼容性与边界约束
- 对外 API 保持稳定，新增字段优先可选，不做破坏式删除/重命名。
- `pkg/common` 结构变更必须考虑 control/data 双端兼容。
- 不得把 OSS 已有基础能力迁移到商业版。
- 不得把内部专用 runbook 或敏感操作文档提交到本仓库。

## 5. 文档同步要求
若改动以下能力，必须同步指定文档：
- 统计/归因：`docs/USAGE_ANALYTICS.md`
- 实时可见：`docs/REALTIME_APIS.md`
- 告警：`docs/ALERTING.md`
- 产品边界：`docs/FEATURE_MATRIX*.md`、`docs/OSS_ENTERPRISE_BOUNDARY.md`

## 6. 安全与隐私红线
- 默认只保留元数据审计；内容留存遵循现有策略开关。
- 禁止提交明文密钥、token、内部地址、个人隐私数据样例。
- Demo 数据必须是伪造数据。

## 7. 提交规范
- 小步提交，单提交只做一类改动（代码/文档/重构分开）。
- Commit message 建议：`feat:`、`fix:`、`docs:`、`chore:`。
- 若改动 API，请在提交说明中写明兼容性影响。

## 8. 常见任务 Playbook
### 新增管理接口
1. `control-plane/internal/api/http.go` 增加 handler。
2. `control-plane/internal/store/store.go` 增加数据读写。
3. 同步 `docs/` 对应说明。

### 新增实时统计维度
1. 优先复用 `UsageEvent`/`AuditEvent` 字段。
2. 缺字段时先在 `pkg/common/types.go` 做向后兼容新增。
3. 补充 `/admin/realtime/*` 文档与示例。
