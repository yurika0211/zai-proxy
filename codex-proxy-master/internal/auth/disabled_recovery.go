package auth

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
)

/**
 * jsonPathFromDisabledCredentialPath 由 *.json.disabled 或 *.json.disabled.N 还原目标 .json 路径
 */
func jsonPathFromDisabledCredentialPath(disabledPath string) (string, bool) {
	base := filepath.Base(disabledPath)
	lower := strings.ToLower(base)
	const marker = ".json.disabled"
	i := strings.Index(lower, marker)
	if i < 0 {
		return "", false
	}
	tail := lower[i+len(marker):]
	if tail != "" {
		if len(tail) < 2 || tail[0] != '.' {
			return "", false
		}
		for _, c := range tail[1:] {
			if c < '0' || c > '9' {
				return "", false
			}
		}
	}
	stemEnd := i + len(".json")
	if stemEnd > len(base) {
		return "", false
	}
	return filepath.Join(filepath.Dir(disabledPath), base[:stemEnd]), true
}

func listDisabledCredentialPaths(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if _, ok := jsonPathFromDisabledCredentialPath(name); !ok {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	return out, nil
}

/**
 * ProbeRestoredAccount 对已回到号池的账号做 OAuth 刷新 + 同步额度校验；失败则删除凭据（不再改回 .disabled）
 */
func (m *Manager) ProbeRestoredAccount(ctx context.Context, acc *Account, qc *QuotaChecker) {
	if m == nil || acc == nil {
		return
	}
	email := acc.GetEmail()
	acc.SetActive()
	if !acc.refreshing.CompareAndSwap(0, 1) {
		log.Debugf("禁用恢复探测 [%s] 跳过：正在刷新中", email)
		return
	}
	defer acc.refreshing.Store(0)

	acc.mu.RLock()
	rt := acc.Token.RefreshToken
	acc.mu.RUnlock()
	if rt == "" {
		log.Warnf("禁用恢复探测 [%s] 无 refresh_token，删除凭据", email)
		m.RemoveAccount(acc, ReasonRestoreProbeFailed)
		return
	}

	rctx, rcancel := m.refreshRequestContext(ctx)
	defer rcancel()
	td, err := m.refresher.RefreshTokenWithRetry(rctx, rt, 3)
	if err != nil {
		log.Warnf("禁用恢复探测 [%s] OAuth 失败: %v，删除凭据", email, err)
		m.RemoveAccount(acc, ReasonRestoreProbeFailed)
		return
	}

	acc.UpdateToken(*td)
	m.enqueueSave(acc)
	qcEff := m.effectiveQuotaAfterRefresh(qc)
	if qcEff != nil && !m.afterRefreshValidateQuota(rctx, qcEff, acc) {
		return
	}
	log.Infof("禁用恢复探测 [%s] 刷新与额度校验通过", acc.GetEmail())
	acc.SetActive()
	m.InvalidateSelectorCache()
}

/**
 * RunDisabledCredentialRecovery 扫描 auth 目录下禁用凭据，还原为 .json 后加入号池并探测；仅非数据库模式有效
 * @returns 本周期尝试恢复（已执行探测）的个数
 */
func (m *Manager) RunDisabledCredentialRecovery(ctx context.Context, qc *QuotaChecker) int {
	if m == nil || m.db != nil {
		return 0
	}
	authDir := strings.TrimSpace(m.authDir)
	if authDir == "" {
		return 0
	}
	paths, err := listDisabledCredentialPaths(authDir)
	if err != nil {
		log.Warnf("禁用凭据恢复: 读取目录失败: %v", err)
		return 0
	}
	if len(paths) == 0 {
		return 0
	}
	sort.Strings(paths)
	log.Infof("禁用凭据恢复: 发现 %d 个待探测文件", len(paths))
	n := 0
	for _, disPath := range paths {
		if ctx.Err() != nil {
			break
		}
		jsonPath, ok := jsonPathFromDisabledCredentialPath(disPath)
		if !ok {
			continue
		}
		if st, statErr := os.Stat(jsonPath); statErr == nil && !st.IsDir() {
			log.Warnf("禁用凭据恢复: 目标已存在，跳过 %s", filepath.Base(disPath))
			continue
		}
		if err := os.Rename(disPath, jsonPath); err != nil {
			log.Warnf("禁用凭据恢复: 重命名失败 %s: %v", filepath.Base(disPath), err)
			continue
		}
		acc, loadErr := loadAccountFromFile(jsonPath)
		if loadErr != nil {
			log.Warnf("禁用凭据恢复: 解析 %s 失败: %v，删除文件", filepath.Base(jsonPath), loadErr)
			_ = os.Remove(jsonPath)
			continue
		}
		m.mu.Lock()
		if _, exists := m.accountIndex[jsonPath]; exists {
			m.mu.Unlock()
			log.Warnf("禁用凭据恢复: 号池已有 %s，还原为禁用名", filepath.Base(jsonPath))
			if err := os.Rename(jsonPath, disPath); err != nil {
				log.Errorf("禁用凭据恢复: 无法还原 %s: %v", filepath.Base(jsonPath), err)
			}
			continue
		}
		m.accounts = append(m.accounts, acc)
		m.accountIndex[jsonPath] = acc
		m.publishSnapshot()
		m.mu.Unlock()
		m.InvalidateSelectorCache()
		n++
		m.ProbeRestoredAccount(ctx, acc, qc)
	}
	return n
}
