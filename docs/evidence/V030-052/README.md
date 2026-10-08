# V030-052 · 密码 Schema 校验器替换证据

基线 develop：`440d171ebca38ff97962aa24f391391fb029b431`。

[PRD](https://app.notion.com/p/3f32f5a9e6488147a531d2c4eebfad7f) · [ADR](https://app.notion.com/p/3f32f5a9e648819eacc2e6cb5fed399b)

## 独立预期

Q13 / FR-018 / 已接受 ADR-001 要求密码是可打印 ASCII 字符串，并具有大小写英文字母、数字、可见标点四类；普通空格保留但不计标点，不 trim、不规范化、不添加额外长度规则。保留原11个合成字符串样本，增加数组、null、布尔、数字、对象五类 JSON 值；仍是原2个测试身份。预期不是从后端实现计算得出。

## 实际时序

1. 原始解释器/原11个样本：2 PASS。
2. 先只追加类型矩阵：Q13 actual true / expected false 的真实 AssertionError，账号项仍PASS。首个新增数组经过旧 helper 的 JavaScript 隐式转换，被当成字符串处理。
3. 观察RED后才替换为Ajv2020，使用同一16样本：2 PASS，输入未改变。
4. 隔离副本增加错误 maxLength=3，破坏已批准的四字符合法输入：1 FAIL/1 PASS，actual false / expected true。恢复原Schema字节后2 PASS。正式OpenAPI全程不改。

环境固定为 Node24.14.0、pnpm10.28.2、Ajv8.20.0。依赖安装使用 frozen lockfile / ignore-scripts。RED不是安装、语法或环境失败；变异失败是实际业务预期断言。

## 可恢复资料

- baseline-test.mjs.txt 与 red-test.mjs.txt 是实际运行的原字节源码，扩展名避免被测试发现规则误收集。
- root-red.patch、root-green.patch 保存真实分阶段补丁。最终完整锁文件在同一提交根目录；package-lock.diff及dependency-comparison.json记录依赖变化。
- execution.json 保存阶段命令、时间、退出码、原始stdout/stderr及其SHA256；包括安装、RED/GREEN、隔离变异与本地全检查，不是重新生成的测试结果。
- recovery-manifest.json记录复制原件的路径、大小与SHA256。check-schema-mutation.py为Root编写的隔离验证脚本，只改临时contracts副本。

复现RED须使用独立基线工作树，将 red-test.mjs.txt 原样复制为 contracts/password-ascii.contract.test.mjs，运行固定 Node 的 `node --test contracts/password-ascii.contract.test.mjs`。复现GREEN使用本目录所在最终提交的源码和lock安装精确依赖，再执行同一命令。不要覆盖仍在使用的工作树。

## 工具边界

只使用Ajv的2020-12入口编译当前实际password Schema Object，不声称Ajv校验整份OpenAPI3.2.1。按OAS格式语义单独登记password为非验证注解；strict开启，没有关闭所有格式检查，没有启用coerceTypes/useDefaults/removeAdditional。工具只在契约测试使用，不加入生产认证。

仅添加一项直接开发依赖Ajv8.20.0及其四项传递依赖；原122个packages、122个snapshots及旧importer条目保持。

## 本地验证与保留警告

契约/底座/治理390 PASS，0 FAIL/SKIP；audit各级漏洞0；typecheck、lint、build和结构检查通过。Lint12条warning的规则/位置/消息与[PR60 CI browser](https://github.com/Hubujiu/WeaveOS/actions/runs/37760742098)一致：license1项、required-property2项、unused-component9项。原有大chunk提示也保留，没有降低门槛。

最终完整CI和Root审查仍待本地候选冻结后执行；最终状态及结果另归档Notion，不提前把本地通过记为develop集成、main批准或部署。
