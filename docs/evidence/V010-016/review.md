# V010-016 实际证据

2026-09-28，Windows Node + Docker Desktop29.6.2，独立016工作树。

- 策略：初始6项目标RED提交83ef3cb，测试及占位源码和命令/退出码保存在policy-*。传输：2项目标RED提交bdb3f5b，源码及输出在bundle-*。
- 真实PostgreSQL：首次临时socket就绪误判造成环境失败，不能作为RED；保留migration-environment-failure及源码。修正TCP就绪后e11bd58实际缺少扩展列（expected1/actual0），再实现Goose Up。最终测试真实确认扩展列存在、原数据42保留、失败SQL不提交99且迁移版本保持1。测试自建容器已删除。
- GREEN：8项策略/传输、1项真实迁移、89项治理/底座通过；不是完整产品验收，最终GitHub检查另核对。
- GitHub environment实际建立为weaveos-production，仅main分支；专用部署Key及固定主机公钥以environment secrets保存。main当前没有分支保护，此部署链本身强制先执行CI和完整验收，不把environment声明成分支保护。
- 服务器迁移基线实际核对：热/冷Goose当前版本均1，两份发布SQL规范LF摘要与兼容清单一致；运行源6e946105f9cb8e77460c54ae2c74ef0d9f021302。受限接收器与root私有state已安装，现有应用镜像/配置未改变。
- 专用Key实际请求`id`且发送空输入：exit1，stdout为空、返回receive阶段安全失败，没有uid输出。证明强制接收命令生效，不会执行所请求shell命令。密钥生成/环境上传均不输出私钥。

后续实际验证：Compose额外include/外部网络/放松用户、只读、capabilities、可信代理规则的缺陷已真实RED→GREEN，最终9项部署检查、90项治理检查通过。命令与源码记录见execution-record.json。

2026-09-28T03:52:19.211Z接收器实际发布已接受源6e94610成功，序号1001；使用服务器已有且与TRANSFER.json一致的两份Docker归档，无构建或格式转换。执行了加密热/冷备份、Goose Up、同镜像重建与公网TLS健康。BFF配置ID c4fff0545e488fd666a0e3ec21dda9ba2b2fb9a791e744bd4a87b924c43b1719，Web a6e9b583e6e6507b09294bf2e9ad25db69e6cc072546cbb20285e2404b3c50e3，与之前运行镜像相同。发布结果deployed；同一包重放在validate阶段failed，序号/镜像未变化。现有monitor无告警、外部默认TLS健康200。真实持有deploy.lock时专用SSH退出1且接收器未启动，验证互斥。

初次OCI下载连续停滞，已停止本任务传输；没有把失败网络当行为RED。上述运行确认使用管理员调用同一受限接收器并读取本机已验收制品，不等同GitHub自动触发成功，也没有在生产注入故障。私有current/ledger/previous恢复资料保留在服务器，凭据不进入本材料。

最终PR head完整CI/OCI转换及main启用仍需现场核对，链接和结果按任务说明记录到PR。自动链路合入main前不生效；开放PR保留工作树。永久TDD资料与运行手册已提交。
