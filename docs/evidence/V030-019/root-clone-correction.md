# Root 测试辅助修正

原 JSON round-trip clone 将 nil json.RawMessage 转为字节 null，DeepEqual 在 Validate 调用前即不同。Root 独立核对 encoding/json 语义及原始字段，改为逐字段 slices.Clone 保留 nil、深复制审批切片，并增加调用前快照相等断言。业务不变性断言没有删除或弱化，生产 graph.go 不为迁就错误快照而改写输入。

修正测试应在原无行为 graph.go 上回放有效 RED（明确为修正后的回放，不伪造原始时间），随后在0baeaf实现上全部通过。新增编译器9项测试另先RED后实现，不把XML静态通过当Flowable引擎通过。
