# 执行证据

本任务为测试先行，当前deliveryState=blocked。产品功能未实现；不能把RED或前置失败写成验收通过。

## 源码时序

- 63c0b0d：先登记任务和已接受来源/冲突；尚无RED。
- 216369c430ddbeb26692d8f2d5e7fc22fa2e84b0：首次测试源码，test-source.zip SHA256 554bcc663db3776e7807804f122e17ba55bc97a03a682442f11ffba1d9dfe5f4。
- aa5efd7427900a25b2e6db387efb8ae09c702c50：补浏览器异常和确定性退出竞态，final-test-source.zip SHA256 1eb92bf2208b621e6ebc8a2f5beeb7ec1f402e2407224651861c4e361375c135。
- 6099099a4afc731872d05e98fe7358819feed81e：Cookie清除接受所有非正Max-Age（原断言仅允许0过严；负值同样表示已过期，未根据被测实现修改），delivery-test-source.zip SHA256 7cc9ef506aab5743bff79c8f641d17e3b182cc50545e0a35923ea314a1cf577e。

工作分支保持真实测试顺序；未提交任何业务实现。ZIP包含tests/acceptance全部源码，squash后不依赖已删分支。归档不是离线Notion替代品。

## 本机与命令

Windows amd64 / PowerShell；Node22.23.1；pnpm10.28.2；Go1.27.1（从go.dev下载到忽略的.work/toolchain）；Playwright1.63.0。浏览器安装最终成功，中间CDN超时不是RED。Docker daemon不可达；没有fixture/真实存储observer，没有伪造Seed。

Go build真实BFF到.work/bff.exe，未修改产品源码。监听127.0.0.1:18749，health/live实际200，readiness/业务仍未装配。`WEAVEOS_API_URL=http://127.0.0.1:18749`。

```sh
node --test --test-concurrency=1 --test-reporter=tap tests/acceptance/api.test.mjs tests/acceptance/integration.test.mjs
```

2026-09-25 11:23 +08:00 实际退出1：116项，0通过，116失败，0skip。其中65个业务状态断言RED，实际404而独立预期400/401/403/415；32个HTTP fixture前置失败与19个observer前置失败合计51 BLOCKED。并非116个目标行为都已观察到RED。原始记录product-delivery.tap保存于本目录；没有凭据fixture内容。

三浏览器选择执行：`pnpm exec playwright test --config .work/red.config.ts tests/acceptance/web.spec.ts --grep 'WEB-01|FR-001/011|FR-007: an anonymous'`，实际9失败/0通过、退出1，均为缺少登录/注册界面与匿名路由未跳转的目标RED。不是浏览器启动错误。全套最终结果在后续记录补充。

本机8080被系统限制；首轮默认配置启动失败为BLOCKED。临时端口配置初次遇到ESM和Windows路径加载问题也不计RED；修正运行配置后才记录上述9个目标失败。浏览器使用真实Vite构建与Go BFF，18750同源代理至18749。临时配置源码保存在本目录，复现时复制回.work使用；不把HTTP底座当最终HTTPS产品验收。

## 工程回归

- `pnpm install --frozen-lockfile --ignore-scripts`：退出0，锁文件未变。
- `pnpm typecheck`、`pnpm build`：退出0。
- `node --check tests/acceptance/{api,http,integration}.mjs`（逐文件）：退出0。
- `node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs`：57/57通过，退出0。
- `node scripts/verify-repo.mjs`、`node scripts/check-tasks.mjs`、`git diff --check`：退出0。
- Go build：退出0；本任务未改Go生产或测试代码。Go race/vet及CI最终结果见后续记录，不预填。
- 全产品GREEN、真实存储/迁移、HTTPS E2E、OpenAPI校验与人工签署：NOT RUN/BLOCKED，详见coverage.md。

本机自动审批曾拒绝组合Start-Process/Stop-Process命令（仅给出blocked by policy，未提供更具体理由）；未强行重试该命令。改用工具管理的独立PTY运行BFF，并通过该session的Ctrl-C停止，执行成功。未向用户要求解除审批限制。
