package auth

import (
	"strconv"
	"strings"
)

type policyPhaseKind byte

const (
	policyPhaseNone policyPhaseKind = iota
	policyPhaseRefreshOnce
	policyPhaseCooldownThenRetry
)

/**
 * httpStatusPolicy 针对某一 HTTP 状态码：先执行 phase，若重试后仍为同一状态码则执行 final
 */
type httpStatusPolicy struct {
	phase policyPhaseKind
	final HTTPErrorAction
}

func parsePolicyPhase(s string) policyPhaseKind {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "refresh_once", "refresh-once", "refresh_one", "refresh":
		return policyPhaseRefreshOnce
	case "cooldown_then_retry", "cooldown-then-retry", "cooldown_retry", "wait_retry":
		return policyPhaseCooldownThenRetry
	case "none", "immediate", "direct", "":
		return policyPhaseNone
	default:
		return policyPhaseNone
	}
}

/**
 * parseHTTPStatusPolicyTable 解析 YAML map：键为状态码字符串，值为 phase/final
 */
func parseHTTPStatusPolicyTable(raw map[string]map[string]string) map[int]httpStatusPolicy {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[int]httpStatusPolicy)
	for k, v := range raw {
		code, err := strconv.Atoi(strings.TrimSpace(k))
		if err != nil || code < 100 || code > 599 {
			continue
		}
		phase, final := "", ""
		if v != nil {
			phase = v["phase"]
			final = v["final"]
		}
		out[code] = httpStatusPolicy{
			phase: parsePolicyPhase(phase),
			final: ParseHTTPErrorAction(final),
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergeRefreshHTTPPolicies(opts *ManagerOptions) map[int]httpStatusPolicy {
	out := make(map[int]httpStatusPolicy)
	def429 := HTTPErrorActionCooldown
	if opts != nil && strings.TrimSpace(opts.RefreshHTTP429Action) != "" {
		def429 = ParseHTTPErrorAction(opts.RefreshHTTP429Action)
	}
	out[429] = httpStatusPolicy{phase: policyPhaseNone, final: def429}
	if opts != nil {
		if t := parseHTTPStatusPolicyTable(opts.RefreshHTTPStatusPolicy); t != nil {
			for c, p := range t {
				out[c] = p
			}
		}
	}
	return out
}

func mergeQuotaHTTPPolicies(opts *ManagerOptions) map[int]httpStatusPolicy {
	out := make(map[int]httpStatusPolicy)
	def429 := HTTPErrorActionCooldown
	if opts != nil && strings.TrimSpace(opts.QuotaHTTP429Action) != "" {
		def429 = ParseHTTPErrorAction(opts.QuotaHTTP429Action)
	}
	out[429] = httpStatusPolicy{phase: policyPhaseNone, final: def429}
	if opts == nil {
		return out
	}
	for k, v := range opts.QuotaHTTPStatusActions {
		code, err := strconv.Atoi(strings.TrimSpace(k))
		if err != nil || code < 100 || code > 599 {
			continue
		}
		out[code] = httpStatusPolicy{phase: policyPhaseNone, final: ParseHTTPErrorAction(v)}
	}
	if t := parseHTTPStatusPolicyTable(opts.QuotaHTTPStatusPolicy); t != nil {
		for c, p := range t {
			out[c] = p
		}
	}
	return out
}
