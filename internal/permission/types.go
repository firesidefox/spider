package permission

// RiskLevel 命令风险级别
type RiskLevel int

const (
	L1Read      RiskLevel = 1 // 只读，无副作用
	L2Write     RiskLevel = 2 // 可逆写操作
	L3Dangerous RiskLevel = 3 // 难以逆转
	L4Destroy   RiskLevel = 4 // 不可逆，影响范围大
)

func (l RiskLevel) String() string {
	switch l {
	case L1Read:
		return "L1"
	case L2Write:
		return "L2"
	case L3Dangerous:
		return "L3"
	case L4Destroy:
		return "L4"
	default:
		return "unknown"
	}
}

// Mode 权限模式
type Mode string

const (
	ModeAsk      Mode = "ask"      // 默认：L3+ 等批准
	ModeAuto     Mode = "auto"     // 自动：L4 等批准
	ModePlan     Mode = "plan"     // 只生成计划，不执行
	ModeReadonly Mode = "readonly" // 只允许 L1
)

// Decision 执行决策
type Decision int

const (
	DecisionAllow   Decision = iota // 直接执行
	DecisionPending                 // 暂停等批准
	DecisionDeny                    // 拒绝执行
	DecisionPlan                    // 返回计划，不执行
)

// Classification 分级结果
type Classification struct {
	Level  RiskLevel
	Reason string // 判定理由
	Source string // "static" or "llm"
}
