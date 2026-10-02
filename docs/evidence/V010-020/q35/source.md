# Q35 独立来源和用户确认

2026-10-01 用户本轮明确：身份与权限模板复用列表+详情；成员及操作记录直接照搬指定 Arca Data Table 完整源码效果；少/零数据留空行填满表体。用户后续答复“连示例中的表格交互也启用”。这些指令取代Q34三表统一、自主原生近似、max360与简化页脚范围。当前代码 writer root，其他Agent只读或仅项目Figma。

实际重新核对远程main6ffba4d、任务a292899与开放PR21；该head的历史CI不作为Q35验证。实际读取项目/PRD入口、人员R3、登录功能、Architecture、ADR002/003/006、Q25物理数据/接口以及历史认证基线；Notion当前集中问题页Q35已记录用户答复，R3§5.8和登录功能界面引用已同步并重读。页面审批、冻结、上线属性未改。

- [人员R3正式§5.8](https://app.notion.com/p/3e92f5a9e64881f4be86c10bc38fff61)
- [集中Q35答复](https://app.notion.com/p/3ea2f5a9e64881f3a36be62911683316)
- [Q25接口字段](https://app.notion.com/p/3eb2f5a9e64881e78a86f51b6e9f94c0)
- [Arca官方示例](https://hubujiu.github.io/arca-ui/#/components/data-table)
- [固定源码](https://github.com/Hubujiu/arca-ui/tree/c0319d887e10775c8968a7d6a451f248858726ec/src/components/motion/table)

官方实际浏览器和c0319d887e10775c8968a7d6a451f248858726ec原始源码独立核对：原Footer33px、按钮24px、PageSize64×24、Motion Checkbox及el-scrollbar。官方本地分页会slice已传数据，末页缩高；这两个行为不能满足WeaveOS真实服务端分页与用户填满要求，允许明确manualPagination/recordCount与fillViewport补丁。列排序仅已获取当前页；页大小默认20可选5/10/20/25/50/100，Q25允许上限100，切换回第一页。不伪造服务端全量排序接口。

空行只是aria-hidden网格，不能有ID/序号/控件/计数/请求；空列表仍有可读空态，分页贴表体底。身份维持同模板卡片/详情、真实说明/影响数字与未保存guard。既有权限、安全活动摘要、搜索筛选、CSRF及版本写保护保持。

只消费22个TS/TSX依赖闭包，加原Tailwind主题/reset、Geist及el-scrollbar；没有装整套组件库。官方源码未找到许可证声明，不伪称MIT。Hubujiu/React-原件、批准登记及approval状态不变；此次直接消费仅基于用户本轮显式指令。

测试的独立预期来自上述确认来源、固定源码和官方浏览器测量；API数据为合成fixture，只证明组件交互，不能冒充真实BFF/数据库产品E2E。新RED/GREEN证据分别保留，Figma同步结果另存。

完整22模块原字节与上游package-lock子图位于upstream-source.zip；vendor-manifest.json记源SHA和项目LF SHA，vendor.patch是零上下文的完整必要项目差异（回放使用git apply --unidiff-zero，路径以归档上游与项目树区分）。未改变原Checkbox/ease/DropdownPanel动效。arca.css消费原theme/light变量/scrollbar，Tailwind4.3.3 preflight/utilities作用于.arca-source后代；全局font-face/theme/property声明与组件实际使用作用域区分，不声称全局无声明。

直接依赖精确版本按固定源锁定；pnpm现有传递解析并非逐字复刻上游npm锁文件：motion13.4.0当前解析framer-motion13.4.6、motion-dom13.4.5，上游快照分别13.4.0/13.3.0；motion-utils均13.3.0、virtual-core均3.17.11。这是原包semver传递解析，源码动效参数保持，三引擎验收核对实际解析的产物；不伪称整个上游依赖锁闭包完全相同。官方npm registry生产依赖审计实际无已知漏洞；默认镜像无audit端点的失败日志另保留，不能当作安全通过。

Figma最终38个白名单文件在导入前逐个验证源SHA；Git文本规范化后用figma/original-import-bytes.zip核对全部原始字节。495节点完整读回、三非sparse上下文和真实截图/恢复记录保留；原始带临时签名资产URL的raw上下文不入仓库。Notion R3§5.8的05:50:38.409Z与集中Q35同步状态已实际重读，审批/发布属性不改。
