# Native version and legacy compatibility GREEN

Root实际读取完整job原始日志，精确head4a030bb61dfecec39cdf377700ac65a9a3af4383。
https://github.com/Hubujiu/WeaveOS/actions/runs/37417288930/job/112118683155

2026-10-06 05:13–05:14 UTC：真实Flowable8/PG18.6独立内部网络、无发布端口。
- 新原生版本8/8，通过严格固定8项gate，无失败/错误/跳过
- 原部署18/18、命令8/8、执行36/36、退回5/5，全套严格门禁通过
- 生产改动仅ControlledBpmn/DeploymentRegistry，未改测试断言
- Java测试与RED快照SHA256相同：9640bc3eea24b8e902fe0e7ffde332a25396f9c40a744d587cf473f0f9d648b1

ci-test-step.txt是原始新8项运行及门禁步骤的时间戳日志截段，不把其称为全部job日志。旧回归及RPC/动作/恢复job由Root另读GitHub事实。完整产品/浏览器和Go仍在跑，不称整个head全绿。本检查点没有catalog接线、流程删除、日志持久或状态镜像精简。
