# V030-020 审计合同缺口与修复记录

初始专项实现按ADR字段写入`application_structure_changed`时，被已发布hot00007 `ck_auth_events_summary`以SQLSTATE `23514`拒绝。该诊断保留在`audit-check-rejection.txt`，不改写历史hot1–15/cold1–4。

Root于2026-10-04更新Notion ADR并提交`719fe9a2b7a178efb6dc416daf2f9dafb726a76f`。保留六个ADR字段；hot00016、cold00005分别增加有限workflow action审计分支，保留旧人员、应用与结构审计CHECK语义。reason_code与action/state一一绑定；完整合同见`audit-contract-addendum.md`。

Root又提交`f0438bf8dae0fd0cace4814bde6f274f11c92a4b`修正冷库测试fixture，为合成归档行提供id和occurred_at，未改变断言或数据库定义。原23502仅是setup失败，保留作历史记录，不作为有效RED。

修复后最终专项：13项`TestRootWorkflowHTTP`、Root热冷CHECK、Root归档复制/冲突保留，全部通过；报告与source SHA见同目录`root-workflow-http-green*`、`audit-hot-cold-green*`和`audit-archive-green*`。
