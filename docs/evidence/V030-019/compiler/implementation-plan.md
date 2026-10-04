## 第二包：受控 BPMN 编译（Root 冻结，须第一包通过后释放）
新增 compiler.go 与 Root 编写 compiler_test.go，不修改统一条件语义。CompileBPMN(Graph,[]appquery.Field,definitionID string)([]byte,error) 先调用 Validate；definitionID 必须canonical非零UUID。返回 XML，不部署、不调用引擎。这里只输出服务生成的固定模板，禁止接受用户 BPMN/XML/UEL。
依据 Flowable 官方 [BPMN Constructs](https://www.flowable.com/open-source/docs/bpmn/ch07b-BPMN-Constructs)：多人任务采用并行 multi-instance；completionCondition 为真会终止剩余实例。本方案由服务端生成固定表达式，不开放用户表达式；实际引擎测试另做。
### 固定映射
definitions 使用 BPMN namespace http://www.omg.org/spec/BPMN/20100524/MODEL、flowable namespace http://flowable.org/bpmn、xsi namespace http://www.w3.org/2001/XMLSchema-instance；targetNamespace 为 urn:weaveos:workflow。只一个 isExecutable=true process，ID p_加definitionUUID去横线。
节点ID为 n_加节点UUID去横线。start为startEvent、end为endEvent、condition为exclusiveGateway；condition的false边设为default，true边含固定表达式 ${route_<节点UUID去横线> == true}。
每个 approval 生成一个 userTask 与一个内部 exclusiveGateway g_加节点UUID去横线。userTask 的 flowable:assignee 为固定 ${approver}；parallel multiInstanceLoopCharacteristics 的 isSequential=false，flowable:collection 为 a_加节点UUID去横线，flowable:elementVariable=approver。
all模式 completionCondition 为固定 ${wf_rejected || nrOfCompletedInstances == nrOfInstances}；any模式为 ${nrOfCompletedInstances > 0}。服务每次推进前设置本次可信 wf_rejected，不能由客户端自行控制。all的单人驳回提前结束该审批节点，any的首个有效操作结束该节点；后来的已失效任务返回冲突，不能改变已结束节点。
approval 原本唯一出边改从内部 gateway 发出，设为其default；userTask到内部gateway额外一条无条件边。gateway到生成的 reject_end 终态额外一条固定 ${wf_rejected == true} 条件边。至少有一个approval时仅生成一个 reject_end；它结束当前实例，不影响业务记录或其他实例。无approval的合法图不生成该额外终态。
原图edge ID按输入索引 e_0/e_1/...；每个approval的内部边 u_<uuid> 和 r_<uuid>。引用和default必须存在，所有XML ID唯一。业务配置的 AssigneeIDs 在此不写死进XML，由受控启动／推进服务按已冻结版本和实时权限生成 a_变量；编辑白名单和字段条件保存在应用定义，不嵌入可执行脚本。
### 条件与安全
Go应用侧使用同一已授权记录快照与统一SQL条件编译器求 route_布尔值，在每次命令发出前重新生成当前记录值对应的条件结果；Flowable不读取任意数据库对象，也不自行eval用户AST。条件节点在需要route变量时没有值应失败并回滚，不默认放行。
生成XML不得含scriptTask/serviceTask/callActivity/listener/timer/async执行扩展或外部DOCTYPE/entity。节点名字与描述不进入此执行格式；UI标签作为非执行metadata存储，避免把可编辑文字误当代码。
字节输出同一输入必须稳定；不可修改Graph/fields。使用encoding/xml安全编码，不拼接未经校验的标识符。XML语法／映射测试仅是编译层验收，必须后续部署到真实Flowable验证all/any/拒绝/条件分支。
