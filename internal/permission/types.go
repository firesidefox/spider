// Package permission defines the risk classification and execution decision
// types for spider.ai's permission control system.
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

// IsValid 返回 RiskLevel 是否在有效范围内
func (l RiskLevel) IsValid() bool {
	return l >= L1Read && l <= L4Destroy
}

// ParseRiskLevel 将字符串解析为 RiskLevel，无法识别时返回 L3Dangerous（安全默认）
func ParseRiskLevel(s string) RiskLevel {
	switch s {
	case "L1":
		return L1Read
	case "L2":
		return L2Write
	case "L3":
		return L3Dangerous
	case "L4":
		return L4Destroy
	default:
		return L3Dangerous
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

// IsValid 返回 Mode 是否为已知枚举值
func (m Mode) IsValid() bool {
	switch m {
	case ModeAsk, ModeAuto, ModePlan, ModeReadonly:
		return true
	default:
		return false
	}
}

// Decision 执行决策
type Decision int

const (
	DecisionPending Decision = iota // 零值 = 暂停，安全默认
	DecisionAllow                   // 直接执行
	DecisionDeny                    // 拒绝执行
	DecisionPlan                    // 返回计划，不执行
)

// String 返回 Decision 的可读字符串
func (d Decision) String() string {
	switch d {
	case DecisionPending:
		return "pending"
	case DecisionAllow:
		return "allow"
	case DecisionDeny:
		return "deny"
	case DecisionPlan:
		return "plan"
	default:
		return "unknown"
	}
}

// ClassificationSource 分级来源
type ClassificationSource string

const (
	SourceStatic  ClassificationSource = "static"
	SourceLLM     ClassificationSource = "llm"
	SourceDefault ClassificationSource = "default"
)

// Classification 分级结果
type Classification struct {
	Level  RiskLevel
	Reason string               // 判定理由
	Source ClassificationSource // 分级来源
}
