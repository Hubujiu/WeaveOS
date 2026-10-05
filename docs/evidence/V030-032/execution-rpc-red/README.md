# V030-032 Root合同 · compile-clean真实RED

Root精确提交fd269704c2bdb7ccee3bb7b8fee9cca95ebe51ab，父07de75ff256f3e78bf04b5730433bfb94778534f。

锁定依赖prepare exit0；生成初次exit1只因新文件untracked，非业务RED；显式add12新bindings后再生成exit0，旧Deployment生成文件无变化。

编译：Go race无测试编译exit0；Java test-compile exit0；互操作脚本静态Go binary和Java编译exit0。

真实RED：Go10顶层1pass/9fail/0skip；Java11为4failures/7errors/0skip；互操作3为0pass/3fail/0skip；Python16为15pass/1error/0skip。四类exit1，原因分别是Root Go无行为constructor、Java继承UNIMPLEMENTED、真实部署后Go constructor占位，以及gate无行为validate。互操作新执行RPC未到达，不能宣称它已工作。

所有报告原始字节保留；case-results.json为派生计数，commands.txt列实际命令，*.exit为真实执行结果。root-source保留Root原稿，compiled-source保留机械gofmt后的实际编译输入及生成bindings；所有14Root文件未改语义，三个Go文件精确等于gofmt(original)，其余11字节相同。原稿与格式化哈希对照见gofmt-equivalence.json。原工具链版本不变，fixture只用b3-postgres合成DB、无公开端口，脚本清理后无fixture容器/网络残留。

仅RED准备；实现未开始，未push或建PR，等Root确认。SHA256SUMS.txt覆盖本证据目录全部其他文件。
