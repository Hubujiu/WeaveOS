# v0.1.0 SSH 隧道服务器运行

目标：43.133.34.48，Debian13 x86_64、2核/约3.6GiB。用户Q15选择SSH隧道；仅合成数据受控运行，不开放公网业务。固定密码重置、无限流及镜像残余漏洞沿用已确认范围，不称生产安全。

服务在`/opt/weaveos-v010`，Compose项目`weaveos-v010-011`。五个服务为PostgreSQL、Redis、BFF、审计维护、Nginx；仅Nginx发布`127.0.0.1:19443`。均`restart: unless-stopped`，沿用已验收内存/日志上限、闭合数据网络、受限应用和维护身份；Docker服务已开机启用。

## 访问

已有SSH配置包含服务器身份；在客户端执行并保持运行：

```powershell
ssh -o ExitOnForwardFailure=yes -N -L 127.0.0.1:19443:127.0.0.1:19443 43.133.34.48
```

打开`https://localhost:19443/login`或`/register`。使用本机自签名证书；可从服务器取回`/opt/weaveos-v010/tls/cert.pem`并核对SHA256后信任，禁止取回或公开私钥。证书有效一年，监控提前3天告警，届时按原运行手册替换并reload。

初始账号仅`bootstrap-admin`。随机密码保存在服务器`/opt/weaveos-v010/admin.json`和部署者本机`.work/deploy/admin.json`，Unix0600/Windows当前SID独占ACL。不要复制到仓库、Notion、日志或公共证据。用户可通过已有SSH在自己的终端读取该私有文件。未导入验收用户、禁用用户、测试邀请码或Session。

## 运行与停止

在服务器操作：

```sh
cd /opt/weaveos-v010
docker compose --env-file .env -p weaveos-v010-011 -f compose.json ps
curl --fail --cacert tls/cert.pem https://localhost:19443/health/ready
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
NODE_EXTRA_CA_CERTS=/opt/weaveos-v010/tls/cert.pem /usr/local/bin/node /opt/weaveos-v010/infra/server/operations.mjs monitor
/usr/local/bin/node /opt/weaveos-v010/infra/server/operations.mjs backup
```

计划任务`/etc/cron.d/weaveos-v010`每5分钟运行探针，每天服务器时间03:15备份冷热库。告警为服务器私有`alerts.jsonl`及`operations.log`，用户通过SSH查看；尚无外发通知。备份存于`backups/`，独立32字节密钥位于`secrets/backup.key`。不自动删除备份，运营者需监控磁盘并另行确认保留策略；它们仍在同机，不具备异机容灾。正式真实数据运行前需另行确定独立备份目的地、RPO/RTO、告警接收与风险范围。

恢复使用原已验收单事务/AES-GCM过程：仅恢复到独立空库，停止应用与维护，恢复后重授角色、切换Session generation，再启动应用。首次部署没有旧服务器发布可回滚；不得用Down迁移假装恢复。源码回滚标签仍在GitHub，候选回滚制品需在CI验证后再搬入。

## 验证边界

配置/目录和维护上下文在本机先RED后GREEN，测试位于tests/governance/server-deployment.test.mjs并由现有CI运行。应用产品验收引用原同制品CI；服务器仅执行成品完整性、迁移和运行健康/页面/资源检查。没有在服务器压测，2核4GB并发容量未知。当前服务器旧构建缓存保持，未修改SSH配置、云防火墙或其他应用。
