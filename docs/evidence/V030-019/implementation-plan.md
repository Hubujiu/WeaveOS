## V030-019 首包精确合同（主负责人冻结）
只改 services/bff/internal/flowgraph/、docs/tasks/V030-019.md、docs/evidence/V030-019/。基线 db475e0dc00e2ac7c18fc5023cbc9039300711ac；不改共享 appquery、dependencies、schema、BFF装配或前端。Root 提供测试和无行为占位。
## 类型
Graph{Version int,Nodes []Node,Edges []Edge}
Node{ID string,Kind string,Approval *Approval,Condition json.RawMessage}
Approval{Mode string,AssigneeIDs []string,EditableFieldIDs []string}
Edge{From string,To string,Branch string}
Validated{Order []string,ReferencedFields []string}
Validate(Graph,[]appquery.Field)(Validated,error)；错误统一可 errors.Is(err,ErrInvalid)，可附节点位置，不返回记录内容。
## 结构规则
Version=1；2至100节点，1至200条边；节点ID为小写canonical非零UUID且唯一。仅start/approval/condition/end；恰好一个start，至少一个end。边端点必须存在，不自连，同一From/To/Branch元组不重复。
start入度0、出度1；approval入度至少1、出度1；condition入度至少1，恰好true和false各一条；end入度至少1、出度0。非condition边Branch必须为空。所有节点从start可达，图无环。条件是二分而不是多个可重叠匹配分支；多路判断可以串接条件节点，保持现有用户范围。
返回Order包含每个节点恰好一次，每条边的起点先于终点，不能修改输入。重复调用同一输入结果稳定；输入数组重排无需保留同一拓扑序，但语义和依赖集合不变。
## 节点配置
start/end不得有Approval或Condition。approval必须有Approval且Condition为空；Mode仅all/any；AssigneeIDs数量1至50、canonical非零UUID且不重复。EditableFieldIDs可空，只能引用传入当前字段，数量至多200且唯一。此校验不代表候选已获授权；发布与执行必须从当前权限来源复核。
condition不得有Approval，Condition必须为非空且非null JSON。复用 appquery.Compile(condition,nil,fields,1) 验证并获得ReferencedFields；不得自行执行SQL或eval，不接受第二套filter语法。字段上下文最多200条，ID唯一canonical非零、Kind只支持现有appquery类型，即使没有condition也必须验证字段上下文。
全部条件所引用字段与节点EditableFieldIDs取去重并集，按UUID字典序返回。依赖用于后续发布引用登记，不产生DDL。本包不处理数据或作权限放行。
## 复杂度
邻接表、入度和可达扫描 O(V+E)，Kahn拓扑 O(V+E)；字段字典 O(F)，条件编译沿用现有受限 AST，依赖排序 O(D log D)。空间 O(V+E+F+D)，不枚举所有路径；100/200/50/200为本版资源上限，错误显式拒绝不截断。
## 验收与边界
Root独立领域样例先RED后GREEN；代码实现者不得改测试逻辑。允许标准gofmt机械格式化并保留哈希。go test -race ./internal/flowgraph、go vet ./internal/flowgraph；纯模块无数据库／浏览器需求。实际BPMN编译、引擎运行、节点权限与完整UI另包验证，不把本次单元测试当流程验收。