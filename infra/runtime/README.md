# v0.1.0 本机 Linux 运行与恢复

本运行配置对应 Notion Q5/Q6：Windows + WSL Docker Desktop 的隔离开发演练。唯一业务入口为 `https://localhost:19443`。用户为本地运行、告警接收及最终验收负责人（Q10）。生产部署、真实域名、异机容灾及公网风险接受均不在本次授权内。

## 完整验证

在本仓库 checkout 中运行，要求 Docker Linux、Git 历史、Node、OpenSSL；Windows 使用 Git 附带的 OpenSSL。没有本机 Go 也可执行。

```powershell
node infra/acceptance/run.mjs
node --test infra/runtime/config.test.mjs infra/runtime/backup-crypto.test.mjs infra/runtime/recovery.test.mjs infra/runtime/monitor.test.mjs infra/runtime/cli.test.mjs infra/runtime/security.test.mjs
node --test infra/runtime/artifacts.test.mjs infra/runtime/storage-version.test.mjs infra/runtime/rollback-security.test.mjs
node infra/runtime/run.mjs
```

运行器拒绝覆盖已有 `.work/runtime/CURRENT.json`。重跑前必须核对该记录的项目和容器，停止这个项目，审查后将完整目录保留到 `.work` 下一个未占用的名称。保留私有文件和数据库卷；不要删除卷、执行 `down -v` 或覆盖失败记录。

运行器构建一次 BFF/前端 OCI 镜像，记录源 commit、镜像 digest、归档 SHA256、二进制摘要，运行受限身份/归档/恢复/回滚、故障和同制品 API/浏览器测试。成功后 `.work/runtime/export/` 只含 `BUILD.json` 和两个 OCI 文件；`.work/runtime/public/` 只含去敏执行结果。结束时停止全部自有容器。

CI 同样执行以上流程；交付工作流等待完整 `check-release`（含用户三项实际签署），下载并交付已经测试的 OCI 文件，不重新构建、不部署生产。

CI仅在GitHub托管Linux临时运行器上清理预装Android SDK以容纳浏览器和扫描数据库；脚本先校验RUNNER_ENVIRONMENT/RUNNER_OS，不在本机执行，不清理Docker卷或测试证据。

从另一目录搬入CI产生的 `BUILD.json` 与两个OCI文件时，先验证并加载原制品，再提供本地解析后的记录。以下命令中的 `BUNDLE/BUILD.json` 替换为实际搬入目录；本地记录保存为新文件，不能覆盖既有记录：

```powershell
node --input-type=module -e "import {importArtifacts} from './infra/runtime/artifacts.mjs'; import {writeFileSync} from 'node:fs'; const r=importArtifacts('BUNDLE/BUILD.json'); writeFileSync('.work/imported-BUILD.json',JSON.stringify(r),{flag:'wx'});"
$env:WEAVEOS_ARTIFACT_RECORD=(Resolve-Path .work/imported-BUILD.json).Path
node infra/runtime/run.mjs
Remove-Item Env:WEAVEOS_ARTIFACT_RECORD
```

这复验同一候选OCI，不重建候选；回滚基线单独从明确已验证源构建。必须使用匹配版本的仓库运行配置，不能将此命令当生产推广。完整搬迁与字节/来源匹配由 `artifact-transfer.test.mjs` 实际验证。

## 查看本地界面

成功演练后，完整私有现场仍在当前工作树。只启动记录中已有的隔离环境：

```powershell
$env:WEAVEOS_RUNTIME_CONTEXT=(Resolve-Path .work/runtime/CURRENT.json).Path
node --input-type=module -e "import {runtimeContext} from './infra/runtime/context.mjs'; const c=runtimeContext(); c.compose('up','-d','--wait','postgres','redis'); c.compose('up','-d','bff','nginx');"
```

访问 `/login`、`/register`。自签名证书仅用于本机演练；自动测试显式信任当前证书，没有关闭 TLS 验证或安装系统根证书。合成测试账号保存在本机私有 `.work/runtime/fixtures.json`，不复制到工单、截图或公开资料。该环境只包含合成数据。

停止时：

```powershell
node --input-type=module -e "import {runtimeContext} from './infra/runtime/context.mjs'; runtimeContext().compose('stop');"
```

## 告警与证书

```powershell
$env:NODE_EXTRA_CA_CERTS=(Resolve-Path .work/runtime/tls/cert.pem).Path
node --input-type=module -e "import {runtimeContext} from './infra/runtime/context.mjs'; import {sampleRuntime} from './infra/runtime/probe.mjs'; import {receiveAlarms} from './infra/runtime/monitor.mjs'; const c=runtimeContext(); console.log(receiveAlarms(c.dir+'/public/alerts.jsonl',await sampleRuntime(c)));"
```

本地接收者查看 `public/alerts.jsonl`：只含时间、接收者和固定告警码，无 Cookie/密码/DSN。探针检查 ready/live、数据库、Redis、Redis容量、容器内存、宿主备份目录磁盘和证书到期。演练阈值为 Redis/内存80%、磁盘空闲10%、证书剩余3天；两天测试证书应主动报警。这些是本地预警阈值，不是容量承诺或生产 SLA。当前按命令/验收运行采样，未安装宿主常驻监控或外发通知。

备份失败通过 `backupDatabase` 的 `alertFile` 写入 `BACKUP`；真实缺源测试证明失败不发布备份且产生告警。容器日志轮转上限为每文件10MB、最多3份；维护进程每小时运行并记录固定消息与计数。

`tls.integration.test.mjs` 生成七天替代证书，验证 Nginx reload 后使用新 CA 的真实 HTTPS；结束时恢复原私钥/证书、重启测试入口并确认原 CA 再次可用。生产 CA 申请/续签服务不属于自签名模拟。私钥先建为 Unix 0600 或当前 Windows SID 独占 ACL，再写入密钥字节。

## 权限、日志归档与数据恢复

应用、审计读取、自动维护、备份和迁移身份独立。应用不是表 owner，不能更新/删除审计或设置 Bootstrap 字段。仅当前有效 Bootstrap Session 可经 `audit-read` 私有 stdin 读取热日志；不新增通用管理 UI。月度归档是独立 PostgreSQL 数据库，保留原12字段、无普通检索索引、无跨库用户外键，满一个日历年自动删除。冷库确认副本一致后才删除热副本，冲突保留原热数据并报错。

备份使用 PostgreSQL18 custom dump + AES-256-GCM，32字节密钥位于 `secrets/backup.key`，加密备份位于 `backups/`；两者为私有不同文件。密钥丢失无法恢复。备份在 Windows 宿主文件系统，脱离 Docker 数据卷，但仍是同一物理机器，不能承受整机/磁盘丢失。演练资料保留到操作者明确审查后清理，没有擅自设置破坏性保留策略。

恢复只接受独立空目标数据库，先认证密文再写入，使用单事务恢复；应用和维护先停，恢复后重新授权受限身份，再为所有 BFF/读取进程统一切换 Session generation，最后恢复流量。真实验证包括禁用状态、密码重置版本、已消费邀请码、冷热事件、旧普通/管理员 Cookie 全部失效。Redis不恢复历史 RDB/AOF。

RPO 为逻辑快照边界：演练刻意在快照后增加一条审计并验证其丢失，不能承诺零丢失。`public/recovery.json` 记录备份和恢复至可用的实测耗时；它不是生产目标。回滚切换已经验证的 BFF/前端组合，不执行破坏性 Down。首个版本只有先前本地已验证代码快照，没有历史生产版本，旧快照不得被视为生产回滚批准。

## 敏感资料与升级边界

只允许上传 `public/` 和 `export/` 中已审查的文件。不要上传 `.work/runtime` 全目录、fixtures、env、私钥、加密备份或 Docker 日志原文。源码机密扫描使用独立的 Git archive，排除工作树外的私有现场。

依赖补丁和镜像摘要以锁文件/BUILD记录为准。Go 源码、所有可修复模块公告、前端依赖及源码机密分别检查。无修复的 OpenPGP 包必须经全应用依赖图证明未链接；基础镜像未修复的 OS 公告仍需记录和用户风险签署，不能称为零漏洞。固定重置密码、无登录限流/绝对会话期限等产品债务沿用已确认 PRD，当前配置不得用于公网生产。
