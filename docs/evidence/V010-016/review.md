# V010-016 实际证据

2026-09-28，Windows Node + Docker Desktop29.6.2，独立016工作树。

- 策略：初始6项目标RED提交83ef3cb，测试及占位源码和命令/退出码保存在policy-*。传输：2项目标RED提交bdb3f5b，源码及输出在bundle-*。
- 真实PostgreSQL：首次临时socket就绪误判造成环境失败，不能作为RED；保留migration-environment-failure及源码。修正TCP就绪后e11bd58实际缺少扩展列（expected1/actual0），再实现Goose Up。最终测试真实确认扩展列存在、原数据42保留、失败SQL不提交99且迁移版本保持1。测试自建容器已删除。
- GREEN：8项策略/传输、1项真实迁移、89项治理/底座通过；不是完整产品验收，最终GitHub检查另核对。
- GitHub environment实际建立为weaveos-production，仅main分支；专用部署Key及固定主机公钥以environment secrets保存。main当前没有分支保护，此部署链本身强制先执行CI和完整验收，不把environment声明成分支保护。
- 服务器迁移基线实际核对：热/冷Goose当前版本均1，两份发布SQL规范LF摘要与兼容清单一致；运行源6e946105f9cb8e77460c54ae2c74ef0d9f021302。受限接收器与root私有state已安装，现有应用镜像/配置未改变。
- 专用Key实际请求`id`且发送空输入：exit1，stdout为空、返回receive阶段安全失败，没有uid输出。证明强制接收命令生效，不会执行所请求shell命令。密钥生成/环境上传均不输出私钥。

待完成：最终CI、实际同制品发布、启用记录和恢复/收尾核验。不得把当前接入准备称为自动同步已经生效。
