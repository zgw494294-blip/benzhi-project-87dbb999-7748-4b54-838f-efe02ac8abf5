# 洞穴石笋采样许可服务

本项目为洞穴保护区管理人员、地质采样研究人员和独立保护复核员提供版本化 JSON HTTP API。服务把一次石笋采样申请从建档、保护基线冻结、候选孔位检查、失败孔位整改，一直推进到独立复核、许可证签发和公开核验。

所有数据保存在本地 SQLite。申请写操作使用 `expected_version` 进行乐观并发控制，并用 `idempotency_key` 和请求指纹支持安全重试。规则检查按不可变批次留存申请版本、保护基线、逐孔结果和规则汇总；独立复核按递增轮次保留当轮孔位修订快照。许可证冻结基线、获准孔位完整参数和最终复核轮次，并使用 SHA-256 规范内容摘要支持独立核验。

## 构建、运行与测试

项目需要 Go 1.22 或更高版本。

```text
go build ./cmd/server
go test ./...
go run ./cmd/server -addr=127.0.0.1:19081
```

默认监听 `127.0.0.1:19081`，默认数据库为当前目录的 `cave-sampling-permit.db`。可用 `-addr` 指定其他监听地址，用 `-db` 指定 SQLite 文件。若未传 `-addr` 且 `PORT` 是有效端口号，服务会监听 `127.0.0.1:<PORT>`，不会默认绑定到所有网络接口。

执行完整、可自行结束的真实回环冒烟流程：

```text
go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
```

`selfcheck` 会启动真实 HTTP 监听，依次建案、冻结基线、提交一个合规孔位和一个侵入禁采区的孔位、运行规则检查、修订失败孔位、定向复检、独立复核、签发许可证并通过公开核验接口验证摘要，然后有界关闭服务。默认情况下它使用内存 SQLite，不会留下业务数据库文件。

## API 流程

所有请求和响应均为 JSON。参与者身份通过 `X-Actor-ID` 请求头传递，写请求在 JSON 中携带当前 `expected_version` 和唯一 `idempotency_key`。可选的 `X-Correlation-ID` 会写入审计事件并原样返回；未提供时由服务生成。

主要路由如下：

- `POST /api/v1/sampling-applications`：创建草拟申请。
- `GET /api/v1/sampling-applications/{application_id}`：读取申请及当前规则结论。
- `POST /api/v1/sampling-applications/{application_id}/baseline:freeze`：由保护责任人在拓扑、空间包含、基准点和配额组合预检通过后冻结保护基线。
- `POST /api/v1/sampling-applications/{application_id}/sites:submit`：由申请人冻结候选孔位方案。
- `POST /api/v1/sampling-applications/{application_id}/checks:run`：运行边界、缓冲、孔距、单孔和累计体积检查。
- `GET /api/v1/sampling-applications/{application_id}/check-batches`：查询检查批次，可用 `site_id`、`rule`、`passed` 和 `type` 筛选。
- `GET /api/v1/sampling-applications/{application_id}/check-batches/{batch_id}`：查询批次及筛选后的逐孔结论；无匹配结论时返回空集合。
- `POST /api/v1/sampling-applications/{application_id}/sites/{site_id}/remediate`：撤回失败孔位或用新编号建立修订谱系。
- `POST /api/v1/sampling-applications/{application_id}/sites:remediate`：原子批量撤回或修订多个失败孔位，并返回建议定向复检范围。
- `POST /api/v1/sampling-applications/{application_id}/sites/{site_id}/checks:run`：对修订孔位执行定向复检。
- `POST /api/v1/sampling-applications/{application_id}/reviews`：由未参与申请的独立复核员逐孔决定 `approve`、`reject` 或 `remediate`；驳回和要求整改必须填写理由，每轮决定都保留在申请查询结果中。
- `POST /api/v1/sampling-applications/{application_id}/permit:issue`：全部孔位获准后签发唯一许可证。
- `GET /api/v1/permits/{permit_id}`：从持久化许可快照重算摘要并分项核验基线、申请版本、终审轮次和获准孔位参数；损坏快照返回 `verified: false` 且不报告有效范围。
- `GET /healthz`：健康检查。

请求体最大为 1 MiB，未知 JSON 字段会被拒绝。业务错误使用稳定的 `error.code`，包括 `validation_failed`、`version_conflict`、`idempotency_conflict`、`invalid_state` 和 `not_found`。研究目的与孔位必要性说明不会写入请求日志。

## 数据一致性

SQLite 迁移由 `schema_version` 管理。每次聚合更新会在同一事务内比较并递增版本、保存聚合及规范化快照、追加审计事件并写入幂等响应。外键维护基线、孔位修订、检查结果和许可证归属；唯一约束保证每个申请最多一张许可证；审计表触发器禁止更新或删除历史事件。
