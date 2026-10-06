# Native scoped deployment RED

Root本人编写测试并读取实际CI原始日志。精确远端head6629ed28da91e108beb67ac34a4fb3b20319fac4，2026-10-06 05:09 UTC。

Run https://github.com/Hubujiu/WeaveOS/actions/runs/37416996765/job/112117769627

命令：bash services/workflow-engine/run-tests.sh clean -Dtest=RootNativeVersioningTest -Dworkflow.reports=ci-logs/native-version-reports test

真实隔离PostgreSQL18.6、Flowable8、锁定Maven/JDK容器。编译与数据库初始化成功，实际8项：1失败、6错误、1通过、0跳过，退出1。合法scoped BPMN在ControlledBpmn.java:70旧version key校验被拒绝；注入故障案例未到AFTER_ENGINE便收到InvalidDeployment，故精确异常断言失败。不是依赖、语法或连接失败。

ci-test-step.txt保留该执行步骤的原始时间戳日志截段（不包括无关依赖下载和上传步骤）；exit从GitHub步骤明确退出1转录。三个Java文件是当时可回放源码；在隔离检出的6629ed2里可直接使用原命令重跑。原始artifact含JUnit XML，ID11391346440；持久证据不只依赖到期artifact。

此时生产Java未改，测试没有变绿。Root在读取上述证据后才允许受控解析接收精确app/flow key及保留旧version key，原allowlist和事务/回执校验不取消。
