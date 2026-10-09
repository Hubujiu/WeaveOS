# V030-061 Root 自审

审查对象：remote/local 4bdef6126dcc8a16a8d6c52a8dd303f8ff5bbc29，tree 0c00d39cfe7af49dca61316adb5e51c02afa1b91。
2026-10-09 13:55 UTC，主助手亲自重新读取服务、可信事务、HTTP/认证接线、索引和相关测试；没有委派。

- 身份来自实际Session与账号authVersion，PersonalRead私有TrustedActor保持同一RR；请求不接受目标actor。
- 只查本人未关闭任务及active实例；固定发布版本roster复核，owner不能代别人处理。closing不提前移除已存在待办，未确认命令不冒称已结束。
- 每个候选当前menu/read/row/field检查先于count/page。完全无read时先过滤，再访问私有表；仍有权的实际存储故障整体失败，不能返回半页。
- 12字段安全DTO，无业务值、完整图、hash、engineID。分页digest仅授权可见摘要，不因隐藏或无关业务字段变化强制刷新。
- QueryContext沿既有会话绑定、域隔离和Redis生命周期。只读RR及10秒预算、64候选批次、最多100返回项约束内存；为准确授权total仍O(N)扫描，不冒称常数成本。真实67应用夹具跨批次验证45可见。
- 新索引只优化actor-leading候选排序，保留旧索引/角色/记录，Down仅删本索引。10万合成候选的访问路径对照不等于生产吞吐压测。
- POST严格body/query与CSRF/ExpectedActor，实际HTTPS覆盖；不改变全局认证语义。
- 原始RED测试和占位源码、失败夹具修正理由、修复前行为RED及修复后完整回归均保存，可在squash后恢复，不依赖临时Actions产物。

本机完整相关回归578顶层/345子PASS，0用例失败/跳过；apprecordhttp无独立测试文件，实际HTTP在cmd/bff。Node513PASS、OpenAPI0错误/13既有警告、vet/build与secret扫描通过。

13:55时精确4bdef的CI与治理已success，产品组仍在运行：迁移/HTTPS/产品、配置/加密/故障/安全已success；同制品恢复/回滚/浏览器及后续仍待。不得在最终产品门禁完成前合并。本文件还需随最后文档候选再次通过精确CI。未改main、未部署。

13:57重新读取：4bdef三项workflow全部success，产品作业已终态成功；两轮实际产品浏览器各141通过。发布用户签署步骤按PR条件skip，未新增签署或部署。jobs/runs及原始日志相关节选同目录保存（节选明确非全日志）。最终纯文档head仍须自身适用CI通过后才能集成develop。
