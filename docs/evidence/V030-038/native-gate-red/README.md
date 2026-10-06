# V030-038 native report gate RED replay

Root本人，2026-10-06 05:05 UTC，Python3。新工作区重建后的真实重跑，不冒称缺失的旧文件已恢复。

命令：python3 services/workflow-engine/native_version_ci_gate_tests.py

结果：9项中合法完整报告被无行为声明拒绝，1 ERROR/8 PASS，退出1。模块已加载，未因环境/语法失败；异常为明确的GateError声明占位。

回放时将本目录两个py快照复制到该提交的隔离副本services/workflow-engine中，保留已有rpc_ci_gate.py，再运行同命令。不要在实际开发树覆盖最终实现。result.txt和exit为真实输出。Java八项真实PG/Flowable测试此时NOT RUN。
