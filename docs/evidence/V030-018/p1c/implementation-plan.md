## P1c 实证补充：并发与真实 COMMIT 响应丢失
Root 于2026-10-04亲自编写 ledger_fault_test.go，共5项新增用例。并发16个真实PG连接验证相同命令只登记一份、两组异负载只有一组可重放、同结果只应用一次。真实PG帧故障仅在读到服务器COMMIT tag后丢弃响应并断连接，分别验证accept和apply：调用方无法确认时，通过另一个连接查询原commandId恢复pending或success；已成功效果不能重做。
同样只在专用PG18.6隔离schema运行，无生产数据。测试不伪造pgx.Tx、不用mock证明事务，也不将单应用库验证说成跨服务证明。校验基线 f04abf825d9f37df1aa12347f2f66c40434a38e0，预期现有实现可通过；如发现缺陷必须保留真实失败，再仅修 ledger.go，测试逻辑不可由实现者修改。
Root查原P1b RED日志实际为14失败／1通过，不能写15全部失败；负向dispatch故障用例对占位也正确返回错误，该用例通过不影响其余14有效RED。请仅修正任务报告的数量，保留原始日志。
全部同包共33顶层测试；并发／响应丢失5项可用 -run '^TestRootLedger(Concurrent|Lost)' 聚焦。验证go test -race ./internal/flowcommands、vet、测试SHA、原始日志。缺环境或socket失败报告NOT RUN，不替代真实结果。Root仍亲自完成验收。
