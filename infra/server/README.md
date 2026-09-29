# v0.1.0 服务器与公网 HTTPS 运行

目标：43.133.34.48，Debian13 x86_64、2核/约3.6GiB。2026-09-28按已接受ADR-004第15节及用户明确授权，开放当前单用户阶段公网入口。固定重置密码、无限流及镜像残余漏洞按该范围接受，不构成多用户生产安全或容量/SLA承诺。

服务在`/opt/weaveos-v010`，Compose项目`weaveos-v010-011`。五个服务为PostgreSQL、Redis、BFF、审计维护、Nginx；仅Nginx发布公网80/443及回环127.0.0.1:19443，其他服务没有宿主机端口。均`restart: unless-stopped`，沿用既有内存/日志上限、闭合数据网络、受限应用和维护身份；Docker服务已开机启用。

## 访问

直接使用 https://weave.hubujiu.site/login 或 /register，无需SSH隧道或浏览器域名回环规则。HTTP80以308保留路径/查询跳转到固定HTTPS域名；BFF同源地址为https://weave.hubujiu.site。原19443仅用于服务器本机健康探针；旧带端口的浏览器登录Origin不再是当前应用Origin。

配置生成使用`serverCompose(existing, { publicTLS: true, publicAccess: true })`；显式公网模式要求公开受信TLS。默认仍保留原SSH拓扑，避免开发/演练被意外公开。公网模式挂载`public-nginx.conf`为Nginx入口配置，不重建镜像；部署时将版本化配置复制到运行目录。

2026-09-27 Q16已完成：用户提供并授权持久保存的主账号CSV在`/opt/weaveos-v010/secrets/tencentcloud.csv`，转换后`acme.env`由续期工具读取，两者root:0600/父目录0700，可供后续已授权操作复用。不要输出、提交、挂载进业务镜像。固定签名acme.sh3.1.6通过DNSPod DNS-01签发Let’s Encrypt证书（YE2，至2026-12-26 05:21:07 UTC），系统信任/域名/密钥/期限检查及Nginx校验reload成功。cron每天服务器时间02:33、14:33检查续期，flock防重叠，首次真实检查成功、尚无需重签；新证书安装到acme-stage后经tls.mjs验证和替换，失败保留/恢复旧文件及私有告警。未安装本地CA。API凭据和ACME私钥/缓存/日志始终受限。

初始账号仅`bootstrap-admin`。随机密码保存在服务器`/opt/weaveos-v010/admin.json`和部署者本机`D:/Workspace/WeaveOS-runtime/V010-011/secrets/admin.json`，Unix0600/Windows当前SID独占ACL。不要复制到仓库、Notion、日志或公共证据。用户可通过已有SSH在自己的终端读取该私有文件。未导入验收用户、禁用用户、测试邀请码或Session。

## 公网配置回滚

V010-015切换前的私有配置保存在服务器`releases/V010-015/compose.before.json`；前后应用镜像ID已比较一致，未改数据/迁移。需要恢复旧SSH访问时，在运行目录将该备份恢复为compose.json，再执行下方Compose命令的`up -d --no-deps --pull never bff nginx`。这会关闭公网业务映射并恢复旧带19443端口Origin，不删除数据库或卷。回滚资料/既有加密备份属于运行恢复资料，保留。

## 运行与停止

在服务器操作：

```sh
cd /opt/weaveos-v010
docker compose --env-file .env -p weaveos-v010-011 -f compose.json ps
curl --fail --resolve weave.hubujiu.site:19443:127.0.0.1 https://weave.hubujiu.site:19443/health/ready
docker compose --env-file .env -p weaveos-v010-011 -f compose.json stop
docker compose --env-file .env -p weaveos-v010-011 -f compose.json up -d
```

不运行`infra/runtime/run.mjs`，不执行产品测试、编译或扫描。Compose不含build，服务器无需Go、pnpm、Playwright或扫描器。`docker compose config`会展开机密，不将其输出到公共资料。禁止`down -v`、覆盖.env/私钥/数据目录或清理已有Docker构建缓存。

## 成品与初始化

应用源main `6e946105f9cb8e77460c54ae2c74ef0d9f021302`；交付run36284110511/artifact10920517529。原始ZIP、BUILD和OCI留存在服务器，TRANSFER记录原OCI摘要与兼容docker-save归档SHA。Docker26经典存储的image ID是OCI config摘要，必须比较原manifest/config字节、revision与RootFS.layers，不能要求其等于containerd manifest ID。仅容器归档格式转换，未重建应用。

既有Goose3.28工具在本机Linux容器构建，在目标专用数据库执行已发布冷热迁移；不修改历史迁移，不在BFF启动时迁移。既有accepted Seed只在本机构造随机Bootstrap；导出唯一管理员、凭据哈希和初始化事件到服务器空库，其他fixture不传输。数据库非空则拒绝初始化。角色使用已验收roles/cold-roles SQL；应用、读取、维护、备份身份隔离，真实秘密独立生成。

## 备份和告警

服务器已有Node22用于轻量运行工具，复用main既有AES256-GCM备份与探针代码。`context.mjs`限定本任务目录/项目；不接受任意主机路径。

```sh
/usr/local/bin/node /opt/weaveos-v010/infra/server/operations.mjs monitor
/usr/local/bin/node /opt/weaveos-v010/infra/server/operations.mjs backup
/usr/local/bin/node /opt/weaveos-v010/infra/server/acme-run.mjs renew
```

计划任务`/etc/cron.d/weaveos-v010`每5分钟运行探针，每天服务器时间03:15备份冷热库。告警为服务器私有`alerts.jsonl`及`operations.log`，用户通过SSH查看；尚无外发通知。备份存于`backups/`，独立32字节密钥位于`secrets/backup.key`。不自动删除备份，运营者需监控磁盘并另行确认保留策略；它们仍在同机，不具备异机容灾。正式真实数据运行前需另行确定独立备份目的地、RPO/RTO、告警接收与风险范围。

恢复使用原已验收单事务/AES-GCM过程：仅恢复到独立空库，停止应用与维护，恢复后重授角色、切换Session generation，再启动应用。首次部署没有旧服务器发布可回滚；不得用Down迁移假装恢复。源码回滚标签仍在GitHub，候选回滚制品需在CI验证后再搬入。

## 验证边界

配置/目录和维护上下文在本机先RED后GREEN，测试位于tests/governance/server-deployment.test.mjs并由现有CI运行。应用产品验收引用原同制品CI；服务器仅执行成品完整性、迁移和运行健康/页面/资源检查。没有在服务器压测，2核4GB并发容量未知。当前服务器旧构建缓存保持，未修改SSH配置、云防火墙或其他应用。

## V010-018 待执行升级

当前在线仍以本文五常驻记录为准。新代码改为四常驻及小时单次任务，服务器接收器/调度尚未升级；执行范围、前置条件、日志参数阻塞和回滚步骤见[升级审阅单](deploy/V010-018-upgrade.md)。
