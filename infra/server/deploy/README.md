# main 自动部署

来源：已接受ADR-004及2026-09-28 Q18用户答复。主机仅运行验收制品，不执行构建、产品测试或扫描。

main push / 手动运行delivery → CI → 完整产品/运行/恢复/扫描验收 → 同份OCI校验与Docker26兼容归档 → main专用environment → SSH受限接收器。环境`weaveos-production`只允许main，不要求重复人工审批。GitHub并发组不取消正在发布的任务；服务器flock禁止重入。新main已出现时旧工作流跳过发布。GitHub排队可替换旧的pending运行，因此目标为最新通过验收的main，不保证每个中间提交都部署。

## 兼容变更

- 迁移文件为`db/migrations/NNNNN_name.sql`或`db/archive-migrations/NNNNN_name.sql`。只能追加；历史迁移在服务器账本锁定。所有变更先按仓库来源/TDD/验收流程完成，再在`compatibility.json`逐项登记规范LF文本SHA256及向后兼容声明。摘要本身不能证明语义兼容，仍需来源审查和真实数据库测试。
- 应用运行配置来自`infra/runtime/compose.json`，Nginx来自`infra/acceptance/nginx.conf`；对应兼容摘要也必须更新。同源公网443、HTTP80重定向、私有数据端口、固定secret挂载和数据卷受策略限制。公网配置由接收器生成，开发/验收仍保持原入口。
- 不在制品中传递`.env`、凭据或证书。新增秘密、改变存储引擎/数据库或Redis配置、基础拓扑、修改接收器本身需要单独实施；接收器不会执行上传脚本或自动替换自身。当前自动配置范围覆盖BFF、审计维护和Nginx运行配置。
- `runId = GitHub run_id * 1000 + run_attempt`用于拒绝陈旧发布；失败后重跑工作流会增加attempt。迁移开始前保守锁定本次全部迁移摘要，即使部分迁移失败也不允许后续改写，应增加修复迁移并重新验收。

## 发布与故障

SSH使用独立Ed25519密钥，仅允许`flock + timeout + receive.mjs`强制命令，禁止PTY和转发；主机公钥通过原有可信SSH读取并固定。私钥仅保存在服务器外的受限恢复目录和GitHub environment secret，服务器只保存公钥。

接收器限制头部和镜像大小、固定文件名，校验源提交/配置/迁移摘要、镜像归档摘要及实际Docker配置ID和层。服务器至少保留3GiB磁盘余量。先校验和导入制品，再对热/冷库加密备份，运行Goose Up，保存旧配置后切换应用。默认信任链检查公网域名的TLS健康、登录页及匿名API拒绝，成功才记录current.json。短暂容器重建会有几秒不可用，不承诺零停机。

验证/备份/迁移失败不切换应用。激活/健康失败恢复旧应用镜像及Nginx/Compose配置，再验证旧版健康；不执行Down、不恢复Redis或旧Session。数据库保留兼容扩展。回滚失败会明确返回`rollback: failed`，不能声称旧版已恢复。

状态和恢复文件位于服务器`/opt/weaveos-v010/deploy-state`，root私有：`current.json`为最后健康版本，`ledger.json`为已尝试迁移账本，`incoming-*/previous.json`为发布前配置，`result.json`记录阶段结果。部署中断留下`journal.json`时后续发布停止，需管理员检查并在同一deploy.lock内恢复previous中的Compose/Nginx及应用、验证健康、恢复previous.current，最后移除journal。保持ledger，禁止数据库Down。不得直接删除journal后继续部署。

服务器只保留部署资料，不上传敏感诊断。GitHub Actions失败就是发布失败通知入口；没有新增邮件/聊天外发。现有监控继续工作。定期审查旧incoming镜像归档及Docker镜像，至少保留当前版本、上一健康版本、未解决失败的恢复资料和数据库备份；本任务不执行全局prune。空间不足会拒绝发布，当前服务继续运行。

## 验证与启用

`node --test tests/governance/auto-deploy.test.mjs`验证策略/传输；`WEAVEOS_TEST_GOOSE`指定已构建Goose后，`node --test infra/server/deploy/migrate.test.mjs`使用真实隔离PostgreSQL验证兼容迁移与事务失败。完整acceptance还实际转换同份OCI制品，服务器无需编译。

接收器由管理员审阅并单独安装至`/opt/weaveos-v010/infra/server/deploy`；首次安装必须核对现存热/冷库迁移版本和文本摘要。代码中的服务器路径、项目、域名、主机身份均为此已授权部署。工作流合入main后才开始自动触发；启用/首次成功的实际记录见V010-016任务，不能把环境和密钥已配置等同自动发布成功。
