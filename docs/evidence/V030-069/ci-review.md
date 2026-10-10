# 精确候选CI核验

- source/head: 3aecf15196a43b4f878852f1cc5d050a52ec1afd
- checked: 2026-10-10 07:58 UTC
- [governance](https://github.com/Hubujiu/WeaveOS/actions/runs/38034116753): success
- [CI](https://github.com/Hubujiu/WeaveOS/actions/runs/38034116762): success
- [product](https://github.com/Hubujiu/WeaveOS/actions/runs/38034116770): success
- 全22 job及完整steps的连接器原始查询事实：ci-source-green.json。
- product job114162470692日志已实际读取：07:45和07:55两轮141浏览器通过，热/冷加密恢复、Cookie换代和同制品回滚、真实Goose迁移通过。最后包装输出 `{"status":"promotion-blocked","backwardCompatible":false}`，这是预期阻止不兼容推广，不能解释为生产部署完成。
- 初始515c候选在PR号尚未写入时preflight任务元数据检查失败。修改pr:78后3aec全部成功；初始失败未进入业务测试，不是产品回归失败。
- 本记录的追加提交仅文档和CI证据；合并前仍对新head重新读取CI，不能以本记录替代最终引用核验。
- TDD源码/无行为占位和原始执行结果均在同任务evidence中；不依赖未来会清理的任务分支或会过期的Actions artifact。
