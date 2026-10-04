## 第三包：真实 Flowable 兼容验收（Root 冻结）
Root亲自新增Go fixture导出测试及Java GeneratedGraphTest的9项业务断言，不委派编写测试。由实际CompileBPMN导出all/any XML，原字节部署至真实Flowable8.0.0和隔离PG18.6。覆盖all须全部同意、any首个同意、两模式驳回、先同意后驳回、最新route变量、外层真实事务回滚、engine-context重启保留待办、缺route不能默认通过。重启仅engine-context，不冒充进程kill或跨服务恢复。
允许从已验证固定B3提交010e2d24fc3570e686e899d80d29d20dcea63fef原字节导入prototypes/flowable-local-tx目录作为测试依赖，不改原15项B3测试或其业务源码。该导入是测试装配，不把旧proof变成产品RPC服务，不冻结生产依赖安全。
精确新增范围：services/bff/internal/flowgraph/engine_export_test.go（Root）、prototypes/flowable-local-tx/src/test/java/org/weaveos/proof/GeneratedGraphTest.java（Root）、该模块src/test/resources/v019/generated-all.bpmn20.xml与generated-any.bpmn20.xml（实际编译器输出）、本任务文档／证据。原模块pom/settings/prepare-build/run-proof/helpers沿用固定内容。允许复用已有镜像与模块缓存，缺失时通过原脚本使用官方固定Maven／PostgreSQL镜像和Maven Central镜像准备依赖；不变更CA、TLS校验或宿主安全设置，不创建正式凭据，不开公网端口。
Go导出命令在services/bff执行：WEAVEOS_FLOW_BPMN_OUTPUT=<绝对worktree>/prototypes/flowable-local-tx/src/test/resources/v019 go test -run '^TestRootExportEngineCompatibilityFixtures$' ./internal/flowgraph。测试无导出目录时仍验证编译，并非skip。记录两份XML SHA256，实际Java测试前后核对一致，禁止手工修生成XML。
运行原prepare-build.sh和run-proof.sh verify。原B3预期15项加Root新9项=24项Java测试；Go全包22项。此处是对已写编译器的独立实证，不伪造RED；若发现真实失败，保存后只修compiler.go，Root测试不改。若Root测试/fixture本身有错误，由Root亲自修正并记录独立原因。
新增测试源码、原B3导入清单／SHA和生成XML保存在仓库，不提交.work、target缓存或任何代理凭据。异常环境报告NOT RUN，不能将依赖失败算功能RED。仅清理本任务专属临时资源。
Root源码审查还发现compiler条件节点当前逐节点扫描所有边，为O(VE)；可在现有测试通过后对false边预建索引改为O(V+E)，保留所有行为断言，属于已测实现的受控重构。复杂度按实际代码记录，不夸大。
