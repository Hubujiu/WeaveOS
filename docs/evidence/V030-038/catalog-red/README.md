# Persisted catalog scope RED

Root实际读取精确4846f69c65b15e718b5c7a96ea2f7fdf6e3014c7的CI job：
https://github.com/Hubujiu/WeaveOS/actions/runs/37418304836/job/112121843448

2026-10-06 05:26:21 UTC，Go1.27.1、隔离PG18.6/Redis8.2.10及完整迁移+原roles.sql。执行go test -race -count=1 -v -run '^TestRootNativeCatalog' ./internal/apprecordservice。

两项均真实运行并FAIL，无skip，exit1：同流程两版key不同且都为version UUID；不同应用各自存储的key均缺app/flow scope。版本UUID不同、旧字节保持等前置断言通过，失败位于稳定scope需求，不是fixture/权限/连接错误。此前15fc4beb缺auth_app的环境失败不算RED，已在任务文档保留。

ci-tests.txt是原日志从两项测试启动到明确退出码的截段，未声称全job日志。测试源码和旧catalog.go完整保存供隔离检出回放；不覆盖正在使用的开发树。读取此证据后，Root才将唯一CompileBPMN调用替换为CompileScopedBPMN(app,flow)。生产发布继续读取保存的原XML，未重新编译或更新旧版本。
