# v0.1.0 验收说明

本版本仍未完成。**底座 CI 通过 ≠ 登录业务验收通过。**

2026-09-25 V010-009已扩展测试先行代码，逐场景覆盖、独立预期纠正与未决项以[coverage.md](coverage.md)为准。新增integration.test.mjs需要真实存储观察器；缺少适配器明确BLOCKED。四项注册表单由用户本次确认。现有HTTP/UI绑定仍待002/006评审，没有宣布全量自动验收完成。

## 自动验证

| 层级 | 文件/命令 | 真实边界与当前状态 |
| --- | --- | --- |
| 仓库/任务/发布规则 | tests/governance、tests/foundation | 可运行；结构规则不是阅读真实性或人工审批证明 |
| Go 平台单元 | services/bff/internal/platform/httpserver | 已有 RED/GREEN；健康/就绪/错误边界，不是认证测试 |
| 底座浏览器 smoke | tests/e2e/foundation.spec.ts | Chromium/Firefox/WebKit，真实构建+Go进程+同源代理，不 mock |
| 产品 HTTP 验收 | tests/acceptance/api.test.mjs | 已写注册/登录/退出/并发/重置/CSRF等独立预期；实现与fixture尚缺，不能判PASS |
| 产品浏览器验收 | tests/acceptance/web.spec.ts | 已写登录/注册/邀请码/密码反馈/刷新/退出；UI尚未实现，预期失败 |
| 真实存储/故障 | V010-003/004/007 的本地与CI集成测试 | 自动化仍待实现，列入阻塞，不转成人工“平台不支持” |
| 完整发布门禁 | scripts/check-release.mjs + v0.1.0.json | 当前必然失败；所有自动/人工项须真实证据 |

API 路径、DTO/CSRF token 位置和前端路由在 tests/acceptance/bindings.json 及测试注释中是**实现前提出的测试绑定**。V010-002 必须依据已接受 ADR 完成契约评审和 OpenAPI 后再实现；不能看到 Handler 写成另一种形式就自动修改测试迎合。UI 可访问名称/密码强度 progressbar 由006对照原型确认。

## Fixture 与隔离

V010-003 在 CI 的专用 PostgreSQL/Redis 上用本地 Seed 命令产生 `.work/acceptance-fixtures.json`，通过 WEAVEOS_ACCEPTANCE_FIXTURES 传递文件路径。包含 admin、user、disabled、resetTarget 的虚构账号数据，各用途独立 invitation（valid/concurrent/rollback/passwordPolicy），以及按浏览器分开的 uiInvitations。每轮创建新数据，不使用生产账号，不把 fixture、密码、Cookie 或数据库转储上传为 artifact。接口返回只能用已确认契约，不能为测试给生产应用添加绕过认证的后门。

API测试默认连接127.0.0.1:8080，可用 WEAVEOS_API_URL指定隔离目标；产品浏览器最终需要同源 HTTPS 测试入口与受信任测试证书，由007装配，不能因Cookie不工作就删除Secure。底座preview仅用于CI/dev，不是生产静态服务器。

## 需求映射与待完成测试

FR-001/005/007/008/009：API与Web登录、恢复、退出；Session过期/故障还须004真实Redis测试。FR-002/003：账号重复、失败不消费、并发同码、单次消费；003补数据库唯一性/事务审计。FR-004/017：URL自动填入和注册不登录。FR-010：禁用登录与已存在会话失效。FR-011/013/015：无自助找回、无隐藏失败锁定、仅Web。FR-012/016：Bootstrap独立入口及重置。FR-014：005/007必须检查日志和实际存储字段且不泄露凭据。FR-018：四类字符拒绝及强度交互。

必须补齐而不能手工替代的自动用例：TTL精确边界/成功活动续期/失败活动不续期、退出和续期并发不复活、Redis/PG不可用不放行、Session固定攻击/CSRF、密码与邀请码存储/日志去敏、Seed幂等与迁移回滚路径、所有接口schema和错误映射。所有这些当前仍阻塞AUTO-DATA/SESSION/AUTH/SECURITY，不能称为已覆盖。

## 需要人工/真实环境的验收

| 项目 | 步骤与证据 | 原因 |
| --- | --- | --- |
| 原型与交互质量 | 产品负责人对照Figma登录/注册节点，验证布局、字级、状态、动效、键盘路径；记录签署与截图 | Actions能截图/无障碍检查，但不能代替用户对设计质量的确认 |
| 实际部署环境 | 验证真实域名/TLS续期、网络暴露、Secret注入与权限；保留去敏命令和结果 | 未提供目标主机/域名/授权，不能在临时runner替代真实环境验收 |
| 灾备与回滚 | 在授权环境模拟故障、从独立备份恢复，测量RPO/RTO和回滚结果 | runner可以做合成恢复测试，但不能证明真实备份和故障域可恢复 |
| 安全债务 | 对固定重置密码及无登录限流作明确发布处置与签署 | 产品/安全决策不能由脚本代签 |

这些不是“GitHub Actions 不支持 E2E”。E2E、数据库服务容器、三浏览器、网络故障注入、构建和测试报告均可自动运行。没有目标环境或功能未实现应写BLOCKED，不得改为PASS。

## PR 与版本验收

每任务先完成自身验收再 squash PR 到 main，远程 main 同名任务全完成才可清理其分支/工作树。整版还必须完成V010-002..008及矩阵所有证据；单独合并V010-001不等于发布v0.1.0。矩阵结构检查无法核验证据内容，评审人须打开对应真实运行和签署。
