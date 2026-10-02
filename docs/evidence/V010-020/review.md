# V010-020 · UI 修复与验证

独立预期：本轮用户截图与Notion人员R3 5.4/Q28、Figma Admin Shell107:256已同步重读。背景应覆盖真实视口，保持320/72侧宽与72/40顶高；仅侧栏收展时20px人员图标不改变y，终态中心x36；48px热区、减少动效及未保存编辑保留。

原因：旧四张材质SVG将1920×1080画板轮廓作为网页固定尺寸；超过基线后顶栏右侧和侧栏底部缺失。侧栏收起删除caption，并把padding-top88改72，菜单/图标向上跳52px；padding瞬时改变也让水平移动瞬时跳变。

实现：AdminMaterial按ResizeObserver给出的外壳/栏尺寸绘制同一连续L面，不拉伸圆角或图标。原22px三次曲线、填充0.76、描边0.42、阴影8/8/15/0.045保持。文字槽位持续存在，仅opacity/blur200ms；宽度和水平padding统一800ms采样弹簧曲线，CSS原生反向和减少动效。原Figma图标资产/尺寸/调用槽位保持，不更改来源组件库，不引入外部UI库。

参考已实际读取的[arca-ui macos-sidebar](https://github.com/Hubujiu/arca-ui/blob/c0319d887e10775c8968a7d6a451f248858726ec/src/components/watermelon/macos-sidebar.tsx)：本项目参考0.8秒spring及0.2秒文字opacity/blur，使用原生CSS表达节奏，不声称复用了Motion物理求解器或整套UI。实际读取时master SHA为c0319d887e10775c8968a7d6a451f248858726ec。

真实TDD：red/保存测试、原PersonnelAdmin/CSS及命令日志。最终有效RED3失败/2通过：大屏材质边缘缺失、逐帧y改变、减少动效y210→158；不是编译/网络错误。259b075先推送再GREEN4cb9b8e。首轮alpha严格194忽略阴影叠加、四状态循环不足，修正测试后在实现前重跑，原日志保留。red/sha256.json使用UTF-8规范化LF，可对Git恢复后的文本重算。

旧固定资产/偏移与144/112菜单预期根据Q28改为实际视口覆盖、196/164高度，不删业务断言。旧立即测量终态用例在800ms动画起点失败，89/90及Firefox/WebKit16/18日志保留；追加可重试width48等待，尺寸/位置断言保持，没有增加固定sleep/整条重试。

所有截图使用synthetic-admin网络fixture，供外壳视觉检查，不是生产账号或真实Session验收。大屏2504×1355与1920×1080四态素材以large/visual-admin前缀保存；实际检查大屏展开/折叠及三引擎截图。API mock仅在组件层；真实产品/存储/恢复回滚由PR完整product流水线另核对。

末段曲线细化：第一次细采样的响应在前半段过快，Windows WebKit单独诊断仅3帧，最早采样已在回弹阶段，缺少可见中间位置（webkit-motion-diagnostic.txt）。CSS替代阴影没有改善（webkit-shadow-css.txt），因此保持原SVG阴影。最终曲线将主要移动均匀分布在800ms内，并减小回弹；WebKit同一断言GREEN（webkit-motion-refined.txt）。没有改阈值或延长采样窗；只为断言添加坐标诊断。最终三引擎适用回归另见motion-final-all.txt。不声称Windows WebKit诊断证明60FPS或CSS阴影优化成功。
