# V030-051 · 覆盖迁移与焦点修复证据

基线：develop `3a33e710461388b27221a030d1e3ea46dfe038eb`。本目录保存可恢复的测试和补丁，不能用已到期 Actions artifact 或以后删除的任务分支代替真实 RED 证据。

## 来源与边界

- [本批 PRD](https://app.notion.com/p/3f32f5a9e648816e95eae1762edef6a6)
- [本批 ADR](https://app.notion.com/p/3f32f5a9e64881bab9b3ec7f8014ba5f)
- [19 项逐项迁移映射](migration-map.md)
- 当前有限 OR-of-AND 编辑器是已批准契约，不恢复旧递归编辑 UI。旧 QueryFilterPanel 和生产 TablePresetManager 的动画控制器不等价。

## 真实 RED 与最小修复

原产品受控慢列表：等待列表请求、打开动画完成后才放行响应；按钮已启用但第一控件未获焦点，Chromium 原实现实际 1 FAIL。较新焦点场景原实现 1 PASS。诊断观测到唯一 focus 调用发生在按钮 disabled/inert 时，之后没有补交接。环境启动失败不算 RED。

- `red-test.ts.txt`：真实 RED 当时完整测试源码，SHA-256 `3f8b584fd9499352fa0c437cc30eb273fc2fba4c2a2f529644d5b349fa9588af`。
- `focus-fix.patch`：从原产品到最小焦点交接修复的完整补丁。
- 原产品 SHA-256 `55465ba8fc342ff48c207a6047000909099963e1ede9a9c6f3eb6b06cb858391`；修复后 `d689202bfbde6e47a8f1ade05f4886f0426aba6e122a57d9630bd7333bf98ed4`。
- 当前正式测试 SHA-256 `1e02475251ca5b23220edd691087ed3cd7652c4244303d350e96d4ecf52caee2`，较 RED 版本补充了两帧等待及正确的像素输入模态准备；未削弱自动焦点预期。

复现应在独立基线工作树进行：将 red-test.ts.txt 原样复制为 apps/web/src/filter-manager.component.spec.ts，使用固定工具运行 filter-manager 专项配置、Chromium、标题筛选 `V051 production manager delayed list focus`、单 worker、零重试。原件应出现上述目标焦点失败；再应用 focus-fix.patch 验证恢复。不要在正在使用的任务工作树上回退源码。

固定环境：Node 24.14.0、pnpm 10.28.2、Playwright 1.63.0；官方镜像 v1.63.0-noble digest `sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27`。使用原超时、workers=1、retries=0；容器 network none，未修改宿主安全设置。

## 截图前提修正

初始及 Escape 后 PNG 相同；outside 只有126×40图像四角4个像素不同，其余5036像素一致。三次受控观测确认实际 focused 均为 true，但初始/Escape 为 focus-visible=true，outside 为 false。最终 helper 先用真实 Tab 确立键盘输入模态，再明确 focus 目标，并断言 focused 和 focus-visible 后全 PNG 精确比较。没有隐藏产品样式、裁剪差异、增加像素容差或减少两个1000ms观察窗口。独立自动回焦断言在 helper 之前。

## 变异证据解释

隔离副本中，忽略真实 Escape 请求使 opening 两例的可见性断言失败；错误打开时长、错误恢复颜色、遗漏 ready-focus 各被实际时长/像素/焦点断言捕获；恢复原件后6 PASS。全部实际 expect 失败，非编译或环境失败。

仅移除 setExpanded 局部守卫的首个变异仍2 PASS，作为存活变异保留，不能称已捕获或证明该局部守卫独立必要。生产控制器还有独立 open/intent/generation 关闭路径；不为杀死局部变异而添加实现耦合预期。

## 退役后实际验证

- 默认入口发现493→490，新增16、删除19；只发现，尚未本地执行全部默认组件。
- Q36 指定入口177→120，完整120 PASS，无跳过；包括108个B2和12个共享Table实例。
- manager入口72→120，完整115 PASS、5 WebKit原生能力SKIP，无失败。
- 两浏览器组均与对应发现身份完全相同，零重试、零 flaky。
- 底座175 PASS、治理158 PASS、契约57 PASS，共390；共享过滤器入口另4 PASS。
- 类型检查、仓库结构和 diff 检查通过。
- 五个 WebKit 跳过为新增0/650ms opening、原生共享几何、native/quick像素。fallback/reduced在WebKit实际执行，不将跳过记作通过。

精确冻结候选的完整CI、Root最终审查及develop集成仍待后续结果。最终状态记录到上述Notion来源，不能据本地定向或发现数字提前宣布全部交付。没有main推广或部署。
