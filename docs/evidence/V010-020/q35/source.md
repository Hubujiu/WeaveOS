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
