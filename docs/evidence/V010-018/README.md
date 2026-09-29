# V010-018 永久证据与恢复

来源与需求→验收映射见任务018和Notion PRD FR02–07 / AC04–11。用户Q19在2026-09-29确认ADR005 D2–D10；Q20仍待参数答复。生产未升级，本目录不代表已经验收或上线。

## 真实RED顺序

以下测试先运行且因目标行为缺失失败(exit1)，再写对应实现。文件名`*.red.txt`是当时测试/旧实现文本快照；`.patch`是差异快照，供squash后重放，不依赖即将删除的分支。源码和输出均不包含真实凭据。表中Git提交用于审查真实时间顺序，重放材料同时在本目录保留。

| 需求 | RED提交/证据 | 缺失断言 | GREEN入口 |
| --- | --- | --- | --- |
| FR04已提交写结果 | 6904476 / committed-red.txt、committed-test.red.txt、service-before-committed.red.txt | 邀请码401/503而非201；重置401/503而非200 | go test ./internal/auth -count=1 |
| FR02可信只读元数据 | 1acfed6 / metadata-red.txt、metadata-test.red.txt、metadata-stub.red.txt、metadata-audit.red.patch | 无可信上下文；实际审计/JSON/header不关联 | go test ./internal/auth ./internal/platform/httpserver -count=1 |
| FR02/03入口 | 5a97f04 / ingress-red.txt、ingress-test.red.txt、nginx-before.red.txt | 外部伪造ID被转发采纳；静态/413无ID | node --test infra/acceptance/ingress.test.mjs（真实HTTPS/PG） |
| FR06生成职责 | 2d3df3e / config-red.txt、config-test.red.txt、config-stub.red.txt | 显式环境生成缺失、参数未拒绝 | tests/governance/ingress-config.test.mjs |
| FR05任务配置 | ef0178b / scheduler-red.txt、scheduler-test.red.txt、compose/plan/policy-before-scheduler.red.txt | 非默认profile、小时Cron、严格command边界缺失 | tests/governance/audit-scheduler.test.mjs |
| FR05执行 | d99d99b / audit-task-red.txt、audit-task-test.red.txt、maintenance-cli-red.txt、maintenance-cli*.red.txt | 宿主函数返回not-run；CLI无--once仍进入循环 | tests/governance/audit-task.test.mjs、go test ./cmd/audit-maintenance |
| FR05安装与探针 | 0f1781f / installed-scheduler-red.txt、installed-scheduler-test.red.txt、probe-red.txt、probe-test.red.txt | 旧拓扑未拒绝；仍采样五个常驻 | 同上两组Node测试 |
| FR07按日/dry-run | 420f32f / daily-logs-red.txt、daily-logs-test.red.txt、daily-logs-stub.red.txt | 无四类文件、无候选清单、未拒绝目录链接 | tests/governance/daily-logs.test.mjs |
| FR07收集与调度 | 20c6ed1、7a98db8 / log-collection-red.txt、log-wiring-red.txt及对应测试快照 | 无stdout/stderr窗口、无配置关联/分钟Cron | tests/governance/log-collection.test.mjs |
| FR07诊断脱敏 | 115411f、ed92d81 / acme-logging-red.txt、log-method-red.txt及对应源码 | ACME原始输出可泄密；未知HTTP方法阻塞收集 | 同上 |

新增函数的初始无行为声明：`runAuditTask`/`collectLogs`返回`{status:'not-run'}`；`configuredLog`返回false；`validateInstalledMaintenance`返回true。快照中的新测试可搭配这些声明与对应旧源码重放。不要将加载失败、依赖连接失败或缺声明误算为产品RED。

## 预期变更依据

`protocol-before-metadata.txt`保留旧测试：ResponseWriter预置头不再是事实源，按FR02改为可信peer传入固定ID，继续断言头/JSON/PG相等。`old-schedule-test.txt`保留被D7明确替代的内部循环测试；以真实单次CLI、任务互斥和调度切换测试替代。`old-server-deployment-test.txt`、`old-monitor-test.txt`保留旧五常驻/所有服务restart/两条flock预期；新需求明确四常驻、维护no restart及分钟日志收集，资源预算和告警阈值未放宽。

## 环境与执行

本机Windows工作树、Node22.23.1、Go1.27.1（本机及固定Linux镜像）、Docker29.6.2；真实PostgreSQL18.6、Redis8.2.10，独立weaveos测试数据库、单独冷库、Redis DB15。日志单测仅用临时合成文件和显式样例时区/天数，不构成生产参数批准；从未执行日志保留删除。

- `go vet ./...; go test -race -p 1 -count=1 ./...; go build -o /tmp/bff ./cmd/bff`在Linux Go容器运行；PG及Redis都用同一隔离网络命名空间的127.0.0.1。首次使用redis服务别名被既有测试环境门禁拒绝，属环境失败，修正为专用loopback Redis后重跑；不弱化该门禁。
- `node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs`；输出governance-green.txt。合同OpenAPI验证由既有CI执行，3.2.1及数据结构不变。
- `node infra/runtime/run.mjs`在d93e682实际运行：单次/互斥/冷库故障、恢复/回滚/备份/扫描和122项API通过，浏览器110/111通过，WebKit“missing digit”page.goto 30秒超时，整次结果failed。原断言和期限不改；同制品单例复查通过，见webkit-navigation-recheck.txt。不能把复查当成整次全绿。
- `node --test --test-concurrency=1 infra/runtime/scheduler-transition.integration.test.mjs infra/runtime/ingress-recreation.integration.test.mjs infra/runtime/logs.integration.test.mjs`使用上述真实制品和隔离存储，4项全通过，见runtime-additional-green.txt。维护二进制/NGINX/存储均真实；调度文件用隔离路径代替宿主/etc/cron.d，不宣称生产Cron已经运行。
- 提交后故障回归commit-faults-green.txt：真实Redis TCP切断；PG线上协议COMMIT已成功但确认报文被丢弃，客户端503、库内一次提交、不重放、不交付虚假成功。此不确定结果原实现已正确，新增回归直接GREEN，没有伪造RED或改行为。

最终head的适用CI和完整产品验收必须重新核对并记入任务文档。纯说明文档、说明性契约段落、交接和证据索引TDD:N/A，以来源/命令/链接复核，不声称测试证明Notion读取或用户批准。
