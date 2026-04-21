package auth

import "strings"

/**
 * HTTPErrorAction 刷新或额度 HTTP 非预期状态时对账号的处理方式
 */
type HTTPErrorAction byte

const (
	/* HTTPErrorActionCooldown 冷却一段时间后再参与选号 */
	HTTPErrorActionCooldown HTTPErrorAction = iota
	/* HTTPErrorActionRemove 从号池与磁盘/数据库删除 */
	HTTPErrorActionRemove
	/* HTTPErrorActionDisable 移出号池并重命名凭据为 *.disabled */
	HTTPErrorActionDisable
)

/**
 * ParseHTTPErrorAction 解析配置字符串，无法识别时默认为冷却
 */
func ParseHTTPErrorAction(s string) HTTPErrorAction {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "remove", "delete", "删除":
		return HTTPErrorActionRemove
	case "disable", "disabled", "禁用":
		return HTTPErrorActionDisable
	case "cooldown", "cool", "wait", "":
		return HTTPErrorActionCooldown
	default:
		return HTTPErrorActionCooldown
	}
}

/**
 * QuotaApplyOutcome ApplyQuotaUsageHTTPOutcome / 刷新 429 处理后的账号处置结果（便于 API 返回）
 */
type QuotaApplyOutcome byte

const (
	QuotaApplyNone QuotaApplyOutcome = iota
	QuotaApplyCooldown
	QuotaApplyRemoved
	QuotaApplyDisabled
)
