# v0.1.0 本地版本验收材料

状态：自动化最终回归进行中；用户 MAN-UI / MAN-OPERATIONS / MAN-RISK 均未签署。[集中答复入口（Q10/Q14）](https://app.notion.com/p/3e62f5a9e648814497c8df6bf27c8724)。[PR11](https://github.com/Hubujiu/WeaveOS/pull/11) 尚未合并，不能据此称整版已完成。

## 界面

规范：[Figma Login 13:2](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=13-2)、[Register 40:2](https://www.figma.com/design/r0mSerkjrPdwJVME658W6a/WaveOS?node-id=40-2)。按 Q12，记住账号、忘记密码、第三方登录入口保持可见、置灰、不可交互；后续再打磨组件和动画。

| 页面 | 桌面 1440×1000 | 移动端 390×844 |
| --- | --- | --- |
| 登录 | [截图](ui/login-desktop.png) | [截图](ui/login-mobile.png) |
| 注册 | [截图](ui/register-desktop.png) | [截图](ui/register-mobile.png) |

截图来自已完整验证的 85c2ee1 镜像空表单，2026-09-27重新读取上述Figma节点截图对照，使用实际本机Edge拍摄。合成账号/邀请码、Cookie 和私钥不进入截图或公开证据。组件自动化覆盖输入、强度、Loading、错误、键盘与路由；三浏览器真实后端覆盖注册、登录、恢复和退出。

## 运行与恢复

[完整运行手册](../../../infra/runtime/README.md)说明前置条件、启动/停止、私有配置、告警接收、证书替换、角色、冷归档、加密备份、恢复与回滚。

- 目标为 Windows + WSL Docker Linux，唯一入口 `127.0.0.1:19443`；核心服务使用闭合网络。
- 应用、迁移、备份、审计读取、自动维护身份分离；仅有效 Bootstrap Session 可读取热日志。
- 冷归档使用独立 PostgreSQL 数据库；按 UTC 月界移出，满一个日历年自动删除，冲突不删除原热记录。
- 真实恢复验证密码重置版本、禁用状态、已用邀请码、热/冷事件和所有旧 Cookie 拒绝；不恢复历史 Redis Session。
- 加密备份位于宿主文件系统，脱离 Docker 卷但仍在同一机器，不能抵抗整机/磁盘丢失。逻辑快照之后的数据可能丢失，测试实际验证了这一点。
- 告警为本地文件接收，当前按命令/测试采样，未安装持续宿主监控或外发通知；自签名证书替换不等于生产 CA 续签服务。
- 回滚只验证前后端已验证代码快照组合，不执行 Down。首个版本不存在历史生产发布；旧快照仅用于兼容演练。

## 安全与供应链

固定重置密码、无登录限流/自动锁定、无绝对最长 Session 生命周期属于现行 PRD。这里只使用合成数据，本地验收不批准公网使用或生产部署。

已修补 Go pgx/x-crypto/x-sys/x-text 与 Vite 公告，源码 govulncheck、所有可修复应用模块公告、pnpm audit、Git archive 的 Gitleaks 均已实际通过。OpenPGP 的 GO-2026-5932 没有修复版本；全应用依赖图必须证明其所有包均未链接，不能用忽略全部模块告警替代此检查。

存储已更新为官方 PostgreSQL18.6 / Redis8.2.10 并锁定 manifest digest。官方来源：[PG版本政策](https://www.postgresql.org/support/versioning/)、[PG18.6发行说明](https://www.postgresql.org/docs/release/18.6/)、[Redis镜像登记](https://raw.githubusercontent.com/docker-library/official-images/master/library/redis)。不对旧私有数据库卷原地升级。

镜像扫描使用 Trivy0.74.0，对实际 OCI 文件扫描，不挂载 Docker socket，不把运行私有配置送入扫描容器。扫描完成和风险接受分别记录。当前镜像报告仍含 OS 漏洞，不能称为“零漏洞”。[完整摘要](scans/summary.json)记录源85c2ee1、四个镜像digest、配置哈希、时间和所有公告；原始报告：[BFF](scans/bff.json)、[Web](scans/web.json)、[PostgreSQL](scans/postgres.json)、[Redis](scans/redis.json)。

最新官方存储镜像的残余可修复公告：PG46项位于入口提权工具 gosu 的内嵌 Go/sys，Redis7项位于 libssl3/tzdata。官方补丁镜像没有消除这些报告；尚未进行逐路径可利用性证明，不将“内部网络”表述为不受影响。BFF/Web还存在基础 OS 未修复公告。用户需对完整清单作本地限定风险决定（Q14）；未答复前 MAN-RISK 保持 pending。

许可证记录：直接前端包见 [frontend-licenses.json](frontend-licenses.json)；镜像原始扫描保留 OS 包和许可证元数据。Go pgx为MIT、go-redis与x/crypto为BSD系列，依据实际模块缓存LICENSE读取。此为组件记录，未宣称完成全部间接依赖的法律合规审查。

## 证据与门禁

[真实 RED/GREEN、测试和原实现快照](red-green.md) 保留失败及环境错误的区别。最终本机结果、镜像摘要和 CI 链接仍须齐备后才提交签署；不以旧提交绿灯替代最终 head。

85c2ee1 的[本机完整运行](local-runtime.json)通过：6项操作、1项DNS、1项实际告警、2项TLS、4项备份、3项制品完整性/搬迁、2项四镜像扫描、25项真实API、30项三浏览器。总运行205.495秒。[恢复证据](local-recovery.json)实测9498ms，备份582ms；[告警接收](local-alerts.jsonl)、[可搬迁制品摘要](BUILD.json)均已保留。该轮回滚用2f93旧快照；随后独立回滚安全测试发现旧快照Go/Vite公告2RED，已把基线更新为完整验证过的85c2ee1，Go/pnpm两项GREEN；更新后的实际组合将由最终CI再次验证。

`node scripts/check-release.mjs` 在三项人工证据缺失时应拒绝发布。最终按用户实际答复回写 Notion，再更新派生验收记录，核对最终 CI 后才能 squash；不代签，不把 PR 草稿当发布。
