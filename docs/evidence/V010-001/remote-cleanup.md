# 远端与本地清理分层 · 独立测试证据

需求依据：任务被远程main同名同ID全完成文档及merged PR验收后删除分支；不同机器工作树不能由GitHub runner假装已检查。远端只检查远程引用，删除其已验收head；本地仍要求本地tip等于PR head且无未提交/未跟踪/未审查忽略文件。远端引用已删除不应阻碍合法本地清理。

先写tests/foundation/remote-cleanup.test.mjs的10个用例，再建立下方最小无行为占位并实际RED。本地测试提交c561d9998e9b6d1bb97734dbd609de90b54da225；命令 `node --test tests/foundation/remote-cleanup.test.mjs`，6通过/4失败，退出1。然后才实现判定，原测试不变，10/10通过，退出0。

测试SHA256：fdf7e5f0bd131543e33dabbc52c2e0ff3770a1235ac69eb7641a669eb6ecc3e0。实现SHA256：c92f9d25f452aaa85f3b78a1b3fbb1bd0780aa8801684cb668875675cdae2386。

squash后的复现方式：复制main到独立临时目录，将scripts/remote-cleanup-policy.mjs替换成以下占位，再运行同一测试。不要覆盖当前主工作目录。

```js
// RED interface-only scaffold; no decisions implemented.
export function remoteCleanupDecision(_input) { return { allowed: false, alreadyDeleted: false, reasons: [] }; }
export function resolveCleanupTip(_localTip, _remoteTip) { return ''; }
```

随后恢复main该实现再执行应为GREEN。此说明保留可恢复的RED源码与正确预期，不把后续复现伪装成原始时间。远程删除操作只有合并后的真实工作流执行才可记成功，纯判定测试不等于已经删远程分支。
