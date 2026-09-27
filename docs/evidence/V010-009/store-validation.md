# STORE 装配与真实验证 · 2026-09-27

来源：用户要求完成PR #3直到可合并。重读PRD（2026-09-27T06:23:54.200Z）、ADR001/002、ADR003/004适用章节、登录/身份、数据设计/DDL（2026-09-26T17:02:38.079Z）/Redis（2026-09-26T17:02:36.979Z）、5个对象和16个相关字段记录。规则未变；旧的缺失fixture改为正式API创建独立用户，STORE14绑定正式seed.userId，未降低预期。

环境：Windows Node22.23.1、Docker Linux、Go1.27.1、PostgreSQL18.6、Redis8.2.10；独立项目weaveos-v010-007-1790514684710，TLS https://localhost:20443。私有环境/fixture/备份/Redis DUMP不提交。Go工具网络EOF与首次PG黑洞超时属于环境BLOCKED，未作RED。最终PG故障使用针对BFF IP的真实拒绝规则，保留本地SQL观察连接。

| 阶段与命令 | 实际结果 | 永久证据 |
| --- | --- | --- |
| `node --test tests/acceptance/storage-observer.test.mjs`，无行为过期占位 | exit1，目标PTTL仍59383而非-2 | store-observer-red.txt / store-observer-red-source.zip |
| 同命令，7项原生变更资格测试，无行为占位 | exit1，7个目标断言失败；TTL/时间/停用/PG和Redis连接/提交故障/过期未改变 | store-controls-red.txt / store-controls-red-source.zip |
| 同命令，真实原生控制 | exit0，7/7 | store-controls-green.txt |
| `node --test tests/acceptance/redis-gate.test.mjs` | 透明转发占位exit1，EVAL提前把before写为after；实现屏障后exit0，1/1 | redis-gate-red.txt / redis-gate-red-source.zip / redis-gate-green.txt |
| `node --test --test-name-pattern='STORE-(0\|1[0-4]\|17)' tests/acceptance/integration.test.mjs` | exit1，17项中16通过，内部事务503≠独立ADR预期500 | store-product-red.txt / store-product-red-source.zip |
| `node --test --test-name-pattern='STORE-15' tests/acceptance/integration.test.mjs`，恢复无行为占位 | exit1，数据库/旧key/generation三项均未恢复 | store-recovery-red.txt / store-recovery-red-source.zip |
| `node --test tests/acceptance/topology.test.mjs`，接入前 | exit1，6通过1失败，runner未执行STORE | store-runner-red.txt / store-runner-red-source.zip |
| `node --test --test-concurrency=1 tests/acceptance/integration.test.mjs`，最终真实栈 | exit0，19/19，无skip | store-product-green.txt |
| `node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs contracts/*.test.mjs tests/acceptance/diagnostics.test.mjs tests/acceptance/topology.test.mjs` | exit0，103/103 | store-regression-green.txt |

上述存储命令设置WEAVEOS_ACCEPTANCE_PROJECT、WEAVEOS_ACCEPTANCE_FIXTURES、WEAVEOS_ACCEPTANCE_COMPOSE、WEAVEOS_ACCEPTANCE_OBSERVER、WEAVEOS_API_URL和NODE_EXTRA_CA_CERTS，值指向同一私有隔离栈。CI由runner自动设置，在Linux19443运行；此证据不授权对部署地址执行。初次恢复后短暂502通过跨Nginx一秒DNS TTL的真实/health/ready稳定检查解决，未重试产品用例或改变业务断言。

业务修复仅区分PG内部语句/提交失败500与连接、资源、停服等依赖错误503；已登记业务错误仍保持400/409。测试工具使用Docker原生SQL/Redis，恢复复用backupDatabase/restoreDatabase/recoverRuntime；没有迁移改写、产品后门、mock成功或门禁放宽。原始RED源码zip可在squash后恢复，摘要见STORE-SHA256SUMS。远端最终head CI必须独立核对，本地19通过不替代完整产品/恢复/三浏览器验收。

Go认证/seed在独立weaveos_store_go_test与Redis DB15执行go test -race -p 1 -count=1 ./internal/auth ./cmd/acceptance-seed及go vet ./...，exit0，见store-go-green.txt。STORE-SHA256SUMS只登记不可变zip源码归档；TAP文本保留原始断言/空白，由Git统一换行，不将其跨平台字节差异误当源码损坏。

