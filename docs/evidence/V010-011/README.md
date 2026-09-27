# V010-011 真实部署与配置证据

独立来源：本轮用户指定服务器仅运行成品，并选择Q15选项1；PRD/ADR004已先同步并读回。无需更改认证/UI规则。所有产品行为沿用main6e94610的已验收OCI。

## RED → GREEN

所有配置测试在Windows本机Node22执行；失败都到达目标断言，不以网络、编译或缺依赖当RED。工作分支先提交测试/声明后实现，快照保留，不仅依赖分支。

| 阶段 | RED事实 | GREEN事实 | 永久快照 |
| --- | --- | --- | --- |
| 配置 | cf23cc4，3/3失败，exit1：缺回环拓扑、持久重启与目录覆盖拒绝 | 493946e，3/3 exit0 | plan-red.mjs / plan-red.test.mjs / config-red.txt / config-green.txt |
| 运维上下文 | 0061d31，3G1R，exit1：缺非法路径拒绝 | d4ab815，4/4 exit0 | context-red.mjs / context-red.test.mjs / context-red.txt / context-green.txt |
| 定时运行 | bf0e6da，4G1R，exit1：缺周期、TLS信任与互斥 | b2ea09f包含GREEN，5/5 exit0 | schedules-red.mjs / schedules-red.test.mjs / schedules-red.txt / schedules-green.txt |
| 私有凭据文件 | 初次Windows客户端登录400（缺password，未创建Session）；b2ea09f，5G1R，exit1：期望独立合成Seed的password字段而实际遗漏 | 修复包装函数与私有文件后6/6 exit0，客户端登录201 | bootstrap-credentials-red.mjs / bootstrap-credentials-red.test.mjs / bootstrap-credentials-red.txt / bootstrap-credentials-green.txt |

预期：Q15回环HTTPS、不在服务器构建/测试；ADR004可恢复运行、日志/资源预算、备份/告警；既有初始化规则必须传合法密码且只初始化Bootstrap。运维ctx测试用命令spy确认实际适配器发送的目标及参数，不将spy当真实备份；真实备份另外记录。6项新配置测试自动纳入现有governance CI。

复核命令：`node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs`，63/63 exit0；`node --test infra/runtime/config.test.mjs infra/runtime/backup-crypto.test.mjs infra/runtime/monitor.test.mjs`，11/11 exit0；verify-repo/check-tasks/diff exit0。这些在本机执行，不在运行服务器执行。

## 部署命令与事实

1. 严格SSH known_hosts只读核查：Debian13.2、2核3665MiB、Docker26.1.5/Compose2.26.1，无容器/镜像/卷，原构建缓存19.35GB保留。
2. 既有`importArtifacts`完整校验原GitHub成品；本机`docker save`为Docker26兼容转移，仅归档转换。TRANSFER记录原OCI摘要、兼容归档SHA/层/source。
3. 本机Docker Go1.27.1编译既有Goose3.28/accepted seed；既有seed仅在本机无公开端口的独立PG生成。只导出唯一Bootstrap行、凭据哈希和初始化事件；其余账号/邀请码fixture不传服务器。本机准备容器停止保留，没有清其他worktree现场。
4. 新目录`/opt/weaveos-v010`0700，scp允许清单私有包（不含fixtures/seed.env），以及原GitHub签名artifact直下；GitHub认证token没有发送服务器，签名URL不保存。
5. 服务器运行`node install.cjs`，先核ZIP、OCI manifest/config，再加载兼容归档，逐项比较image config ID/RootFS.layers/source。全部成功后启动PG/Redis。原ZIP摘要4f8594b0…5019。
6. 初次将dotenv作为Shell读取，URL里的&误解析，Goose未应用迁移；这是运行命令错误，不计业务行为RED。检查Goose可执行、auth schema缺失后，`node resume.cjs`直接按dotenv数据传docker exec环境值；冷热Goose均v1 applied。未重建、清卷或重复创建库。
7. Bootstrap SQL只允许空用户表；真实结果users=1/admin=1、invitations=0。按既有roles/cold-roles授权，秘密随机生成；同源来源localhost19443、generation新值。
8. 五服务`unless-stopped`，Docker开机enabled；curl受信当前CA确认ready200。首次私有密码文件因使用大写Password而遗漏实际小写password；修复前文件保留，新文件0600替换，不改数据库哈希。
9. Windows隐藏SSH隧道进程68140已启动；`NODE_EXTRA_CA_CERTS=…/cert.pem node .work/client-check.mjs`确认页面/资产/health200、实际管理员登录201、身份200、退出204。只保留状态，无密码/Cookie；这属于客户端运行确认，不在服务器安装/执行产品测试。
10. 复用原AES-GCM与probe/monitor：服务器首次冷热备份160/153ms、运行确认后再备份151/153ms；受限备份角色。测试过的cron配置每天03:15 + 每5分钟，flock避免重叠；cron active，首次计划采样operations.log为0字节（无告警），文件0600。无外发通知。

`install-original.cjs`、`prepare-original.mjs`和`migration-continuation.cjs`保留当时真实操作源码，包含已披露的历史缺陷。它们是证据，不能盲目作为新环境安装器执行；没有秘密字面值。正确包装入口为最终bootstrapCredentials，最终运维配置为infra/server。

## 运行边界

installed.json、client-runtime.json、server-runtime.txt、transfer.json记录真实事实。五容器最后空闲采样约99MiB，系统可用2372MiB、磁盘14GB；没有在2核4GB压测，不承诺并发能力。数据/备份/密钥仍同机，自签名TLS、文件告警；无这台服务器的恢复演练或旧服务器发布回滚。原同制品CI的恢复/回滚不是本服务器执行证据。

私有资料在本机.work和服务器0700目录，不入Git。SHA清单只覆盖公开证据，不含真实凭据。审批/冻结/上线状态保持；发布仅为SSH受控访问。最终任务接受仍需匹配最终head CI及remote main同ID/merged PR。
