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

## 全套浏览器最终执行

`pnpm exec playwright test --config .work/red.config.ts tests/acceptance/web.spec.ts --workers=3 --timeout=10000`，2026-09-25 11:20–11:25 +08:00，实际退出1，三引擎共105失败/0通过/0skip。27项缺fixture、3项缺Redis observer，合计30个显式BLOCKED；其余75项因页面尚无产品UI而失败，其中深层交互常在fill前置步骤终止，不能说75个目标业务都已RED。前述选择性9项已确认到达目标断言；全量不是产品验收通过。

本轮并行只作用于没有fixture、没有依赖故障注入的空UI RED采集；接入真实故障/恢复后必须按coverage.md串行或为每worker装配独立全栈。10秒是本次已知空UI采集的有界超时，默认产品配置30秒未修改。原始browser-all.txt在本目录，可核对每项失败位置。前置加载/编译/端口失败均未冒称目标RED。

补充：本机Go test ./...和go vet ./...实际通过。Go race结果单独登记。GitHub草稿PR #3；21a33cc的Repository governance运行36090299985已success，CI运行36090299966当时pending；后续head必须重新查询，不拿该结果当最终PR已验收。

## 失败诊断去敏修复（独立RED→GREEN）

最终自查发现断言helper用assert.equal(data,null)会在错误data含密码时把整个实际对象打印出来。先添加DIAGNOSTICS-01复现：提交5bba8fe，实际1失败、退出1；diagnostics-red-source.zip及diagnostics-red.tap保留原始版本和断言原因。随后改为布尔断言，Cookie与浏览器Cookie数组断言也只输出布尔/数量，提交48427760278fb75e7040b7eb576c47c53e7a6d15；实际1通过、退出0，diagnostics-green.tap保留证据。此GREEN只属于测试诊断helper，未实现任何认证业务。

最终可恢复测试源码verified-test-source.zip，SHA256见SHA256SUMS。2026-09-25 11:28 +08:00，对该源码执行`node --test --test-concurrency=1 --test-reporter=tap tests/acceptance/*.test.mjs`：117项中1个诊断helper测试通过，116个产品用例失败（65目标RED、51前置BLOCKED），0skip，退出1，见product-verified.tap。原有57项回归再次通过；TypeScript通过；Go race与vet实际退出0。浏览器全量结果对应aa5efd7源码；后来只修改Cookie失败输出以避免泄露，未重跑全浏览器，类型检查通过，不把旧浏览器执行称为最终源码的新运行。

仍须007对Playwright错误上下文/报告做全量去敏装配，本PR没有声称关闭trace就保证所有报告无凭据。本轮没有真实fixture，归档仅包含人工合成测试值，不含会话或真实邀请码。源码/证据归档不包含.work依赖、浏览器下载或二进制BFF。
