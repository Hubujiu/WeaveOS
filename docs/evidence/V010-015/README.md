# V010-015 公网入口证据

授权：2026-09-28用户要求完成剩余公网更改；已实际读取已接受ADR004，第15节允许单用户公网和当前安全债务。PRD运行范围及Q17已同步读回。

## 测试先行

- d15526d为真实RED提交，早于实现7b05a7c；原始测试/plan/无行为nginx占位保存在本目录，red.json记录本机命令、环境、时间和exit1。
- `node --test tests/governance/public-entry.test.mjs`：4目标RED/1回归GREEN。独立预期：公网80/443、固定HTTPS重定向、同源Origin、可信代理和认证不缓存、内部端口不公开、原回环健康与续期保持。
- `node --test tests/governance/public-entry.test.mjs tests/governance/server-deployment.test.mjs`：29通过；`node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs`：86通过。verify-repo/check-tasks/diff通过。
- config文本断言不冒充运行Nginx：服务器实际`nginx -t -c /tmp/weaveos-public-nginx.conf`及激活后的`nginx -t`均成功；真实重定向、页面/API/资源、TLS/CSRF/退出另由公网客户端验证。

## 实际部署与客户端

2026-09-28 11:01 +08:00：切换前Compose `config --quiet`通过；冷热加密备份145/161ms成功；保存旧Compose和plan到服务器releases/V010-015。仅重建BFF/Nginx容器实例，`up -d --no-deps --pull never bff nginx`；比较前后镜像ID一致，没有服务器编译、产品测试、镜像扫描或数据库迁移。

首次PowerShell多行管道带CRLF，backup参数尾部CR导致拒绝操作；改用独立SSH调用后备份成功，部署脚本用LF传输。此环境失败不是RED。7b05a7c首轮CI治理失败是任务PR字段仍null，在PR16创建后补登记，不放宽门禁。

client-runtime.json：Windows HTTPS直连43.133.34.48公网443、原域名SNI与默认CA验证，无SSH/忽略证书。login/register/live/ready200、构建JS200、匿名401、登录201、Secure/HttpOnly/SameSite与no-store、身份200、跨站退出403、正常退出204、旧会话401。client-check.mjs只输出状态，不保存密码/会话；本次只创建并撤销一个既有管理员会话，不注册/重置账号，不执行产品验收fixture。

Chrome经正常域名打开/login、点击/register、HTTP地址自动跳到HTTPS成功；isSecureContext=true、origin为标准HTTPS，标题WeaveOS，14个页面/资源请求均200，无控制台错误或警告。CLI专用浏览器已关闭，临时snapshot不入永久证据。未更改系统hosts、代理配置、根信任。

本机VPN的fake-IP解析把域名映射到198.18.1.233，curl普通解析失败；显式使用真实公网IP并校验域名证书成功，Chrome正常访问成功。裸TCP连接在本机TUN下各端口均立即建立，不能据此判断服务器内部服务暴露；public-ports.json保留此不充分观测及说明。实际宿主ss/容器映射（server-state.txt）仅80/443公网、19443回环，数据/BFF无映射；公网19443 TLS握手失败。不要把TUN握手当真实远端服务可达。

服务器monitor实际exit0/无新告警，ACME renew实际passed/exit0（证书未到续签窗口）；既有5分钟探针、每日备份和双日DNS-01续期计划保留，未改密钥和云防火墙/DNS。证书有效至2026-12-26，见server-state.txt。

## 边界与恢复

沿用服务器原main6e94610应用制品；PR15界面工作和main后续认证改动没有顺带发布。当前单用户风险范围、同机备份和无SLA按ADR004，不宣称生产级安全/容量。此次未跑服务器负载/故障/恢复演练。当前配置的恢复命令与私有备份路径见infra/server/README.md。开放PR保留分支/工作树，最终head CI/合并事实另行核对，不预填accepted。
