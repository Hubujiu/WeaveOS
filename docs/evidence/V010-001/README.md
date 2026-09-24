# V010-001 · 可恢复的测试证据

## FND-05：squash 后的证据要求

来源：用户要求squash与验收后删除branch/worktree，同时保留跨Agent可接手的任务依据。要求实现前测试源码/最小接口占位可从main恢复，不能只引用待删分支或到期artifact。CI source.bundle只含Git对象/引用，不打包.git/config、凭据或本机工作目录。

本要求先由pipeline.test.mjs新增两项验证；本地实际RED为3通过、2失败，退出1。随后才增加source bundle与RED源码归档步骤。

## 原始执行与回放必须区分

本任务原始RED提交：4ba15c39c4828d481e386f4e7fa402f024cd5844。原始本地Node21失败；Go修正无关编译错误后6组目标行为失败。之后实现，原来的正确预期未因实现而降低。

后续独立测试阶段：发布门禁1通过/3失败→4通过；清理后验收查询19通过/1失败→20通过；制品证据3通过/2失败→归档后重新检查。新增测试是新需求/缺陷用例，不能宣称整份测试文件从未改动。

`red-source.tar.gz`保存远程真实RED提交的源码；`replay.json`、`replay-node.txt`、`replay-go.txt`是GitHub runner之后重放的结果，**不是伪造原始执行时间**。`green-source.tar.gz`保存归档时的实现源码与新增测试，便于查看差异；不表示整版登录已完成。SHA256SUMS核验归档。

重现：在独立临时目录解压red-source.tar.gz，然后运行 `node --test tests/foundation/*.test.mjs` 和 `(cd services/bff && go test -count=1 ./...)`。预期因行为未实现而失败。不要在当前main目录覆盖已交付代码。

产品HTTP用例已在独立本地18743端口对真实BFF执行匿名/伪造身份/错误密码/缺失及无效邀请码5项；宿主存活200，产品端点实际404，因此5项目标断言RED。其余用例依赖待实现的Seed/业务，未报告通过。

最终GREEN以任务记录及对应PR最终head CI为准。Actions浏览器/覆盖率artifact可能到期，但上述源码归档、任务文档和回放摘要留在main。任何未来任务也必须保留自己的证据，而非引用本任务证明自身TDD。
