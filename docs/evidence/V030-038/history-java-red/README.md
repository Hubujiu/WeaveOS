# Native node history authority RED

Root原始CI核对：精确ed27da92806fa4c2236cabb717ceecd2e356d13f，2026-10-06 05:38 UTC。
https://github.com/Hubujiu/WeaveOS/actions/runs/37419396539/job/112125211233

真实Flowable8/PG18.6，原生版本八项仍全绿；新增历史八项7FAIL/1PASS、0ERROR/0SKIP、exit1。原生历史在同事务存在，但旧代码仍写影子行；清空旧副本使合法退回no_effect；伪造副本使未走过节点被接受；缺失历史配置未被拒绝。跨实例历史隔离原本通过。不是依赖/编译/数据库故障。

ci-test-step.txt为实际历史步骤原始日志截段，exit按明确步骤结果转录。修改前ExecutionRegistry、完整原fixture和测试源码保全；可在隔离检出的ed27da9运行同run-tests.sh命令重现。

读取RED后才删除运行代码的重复INSERT并改用原生同process/userTask/节点存在性查询；新隔离fixture不再建旧表，已有真实数据库未DROP/清空。原36测试中两处影子表数量断言按事先Notion计划换为原生事实：失败回滚后零历史、启动后存在审批节点且两名审批任务；其他业务断言不改，新增八项测试保持原字节。
