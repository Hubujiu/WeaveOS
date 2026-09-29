# V010-018 接收器、调度与日志升级审阅单

本文件为待执行方案，不是已经部署的记录。依据PRD FR02–07、ADR005 D2–D10及Q19；Q18把接收器自身与拓扑升级排除在普通自动发布之外。Q20未答复前不得安装自选日志时区或删除策略。纯运行说明TDD:N/A；对应配置、真实存储、切换和回滚测试见任务018。

2026-09-29 20:24 +08只读核查：服务器运行main `3d7fa2412fbd44f8e5e9031953557fdde8f5a8f8`，`current.json.runId=36432208761002`，五个常驻容器。原Cron为探针、备份、ACME三条；无`/etc/logrotate.d/weaveos-v010`或运行日志按日目录。执行升级前必须重新读取，不能用此检查点覆盖后来发布。

## 审阅内容和制品

- PR19最终通过CI/完整验收的同一head源码和OCI制品；接收器不接受上传程序，也不会自动升级自身。
- 管理员单独安装本PR的`infra/server/{context,operations,plan,acme-run}.mjs`、`infra/server/deploy/{receive,policy,configuration,bundle}.mjs`及其既有依赖；共同规则`infra/runtime/nginx.mjs`，任务`audit-task.mjs`，日志`daily-logs.mjs`、`log-policy.mjs`、`log-collection.mjs`，探针`probe.mjs`与告警`monitor.mjs`。保留既有备份/证书/迁移代码及依赖。逐文件核对CI源码包SHA与批准head，不在服务器编译或运行测试。
- 运行配置仅将现有`audit-maintenance`改为`profiles:["maintenance"]`、`command:["/app/audit-maintenance","--once"]`、`restart:"no"`。保留已安装的镜像、数据库、数据卷、凭据、generation及其他服务配置。旧已验收维护二进制也支持`--once`，因此可以先升级调度，再正常发布新应用制品。
- BFF/Nginx必须成对发布和回滚：旧Nginx未覆盖外部Request-ID，不可单独推广新Go元数据规则。当前在线Nginx配置只在对应新应用发布时切换。
- Q20答复后才生成`/opt/weaveos-v010/log-policy.json`，字段为`timeZone`、`keepDays`、`deleteEnabled`。目前代码仅接受`deleteEnabled:false`，实际删除接口被禁用；确认删除方案后还需完成相应RED/GREEN与实际隔离清理验收。

## 升级顺序

1. 使用既有管理员SSH身份，在`/opt/weaveos-v010/deploy.lock`内串行操作。重新确认无未完成部署journal、当前head/镜像/配置、Cron、四个核心服务健康及冷热备份成功。与未完成任务重叠时停止。
2. 在root私有`/opt/weaveos-v010/releases/V010-018/`新目录保存旧Cron、Compose、Nginx配置、接收器及上述运行脚本、当前发布记录和文件摘要；已有同名目录则停止，不覆盖。备份、密钥、原始配置只留在私有恢复目录。
3. 暂停本项目Cron文件，保留可恢复副本；等待已有备份/维护操作结束。用旧Compose停止维护容器，确认退出再移除该已停止容器。不能同时留下内部常驻循环与小时Cron。
4. 安装已审阅脚本及上述任务配置；运行Compose `config --quiet`，不输出展开秘密的配置。普通接收器的`validateInstalledMaintenance`必须通过；存储服务配置与升级前逐项相同。普通`up -d`仅四个常驻服务。
5. 手动运行`node /opt/weaveos-v010/infra/server/operations.mjs audit`。成功时退出0并写`audit-status.json`；失败时退出非零，记录`AUDIT_MAINTENANCE`固定告警，停止推进。单次Go上下文5分钟，宿主命令上限330秒；超时后确认剩余run容器已经退出，不能只根据宿主退出推断任务消失。
6. 安装Q20明确的日志配置；运行一次`operations.mjs logs`、`operations.mjs monitor`，核对四类日志路径和脱敏字段。用`serverSchedules({publicTLS:true})`生成并审查`/etc/cron.d/weaveos-v010`（root:0644，尾部换行），恢复调度。每小时0分维护，每分钟持flock收集日志，原探针/备份/证书周期保持。维护互斥由真实PG advisory lock负责，重叠失败有本地记录。
7. 核对下一小时实际回执及容器退出；再由原main流程发布已验收BFF/Nginx成对制品。发布继续校验固定镜像、迁移账本、备份、健康与失败恢复，不改generation，不执行Down。不把本地加速演练当成生产小时任务已经运行。

## 回滚顺序

先暂停新Cron并等待/终止本项目仍运行的单次任务，确认数据库锁释放。恢复私有快照中的旧脚本、Compose、成对BFF/Nginx配置与已验收镜像，启动旧维护容器及应用，确认五个服务、可信TLS健康和匿名API拒绝后，最后恢复旧Cron。旧Cron不得包含新小时维护条目。数据库不Down、不恢复旧Session、不切generation。已写按日日志保留，不能用应用回滚撤销已删除数据；本轮尚未启用删除。

本地`infra/runtime/scheduler-transition.integration.test.mjs`实际使用旧常驻二进制、当前单次二进制和真实PG演练5→4→5→4；Cron文件用隔离文件代替`/etc/cron.d`，没有声称本机Windows已经运行宿主Cron。生产安装、首个定时回执和生产回滚均待单独授权/实施。

## 日志责任和限制

运行负责人查看`logs/access/YYYY-MM-DD.jsonl`、`application/`、`operations/`、`metrics/`；日期由明确时区决定。访问仅保留路由形状、ID、方法、状态和字节；应用仅固定消息和关联ID；运维仅操作与结果；指标仅有限数值。认证审计冷热表不属于运行日志。

Docker继续按10MiB×3轮转。分钟收集采用持久时间窗口，处理stdout和stderr；进程中断后重试可能重复已写前缀，连续高流量或停机超过容器环形日志容量可能有缺口，不宣称无损日志平台。收集失败不推进游标，运维按失败回执排查。既有operations.log、alerts.jsonl、acme-private.log及备份/密钥/发布恢复资料均不在清理允许路径，不回填或删除历史私有日志。

`operations.mjs retention`只预览四个固定目录下日期文件；缺少保留天数或时区则拒绝。`--apply`当前始终拒绝。链接目录/文件被拒绝，其他目录和不匹配文件保持原样。Q20确认前，不能把上述参数化能力写成留存需求全部完成。
