// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/**
 * 脱敏规则管理页面 JS
 */
class DesensitizePage {
    constructor() {
        this.rules = [];
        this.builtin = [];
        this.editingId = null;
    }

    async load() {
        await Promise.all([this.loadRules(), this.loadBuiltin()]);
    }

    async loadRules() {
        try {
            const data = await API.get('/desensitize/');
            this.rules = data.rules || [];
            this.render();
        } catch (e) {
            console.error('Failed to load desensitize rules:', e);
        }
    }

    async loadBuiltin() {
        try {
            const data = await API.get('/desensitize/builtin');
            this.builtin = data.builtin || [];
            this.renderBuiltin();
        } catch (e) {
            console.error('Failed to load builtin rules:', e);
        }
    }

    render() {
        const container = document.getElementById('desensitize-custom');
        if (!container) return;

        const categoryIcon = {
            'PHONE': '📱', 'IDCARD': '🪪', 'EMAIL': '✉️', 'BANKCARD': '💳',
            'IP': '🌐', 'APIKEY': '🔑', 'NAME': '👤', 'COMPANY': '🏢', 'CUSTOM': '⚙️',
        };
        const categoryLabel = {
            'PHONE': I18n.t("common.catPhone"), 'IDCARD': I18n.t("common.catIdCard"), 'EMAIL': I18n.t("team.email"), 'BANKCARD': I18n.t("common.catBankCard"),
            'IP': I18n.t("common.catIp"), 'APIKEY': I18n.t("common.catApiKey"), 'NAME': I18n.t("common.catName"), 'COMPANY': I18n.t("common.catCompany"), 'CUSTOM': I18n.t("common.custom"),
        };

        let html = '';
        if (this.rules.length === 0) {
            html = `<div class="empty-state"><p>${I18n.t('desensitize.noRules')}</p><p class="hint">${I18n.t('desensitize.addRuleHint')}</p></div>`;
        } else {
            for (const r of this.rules) {
                const icon = categoryIcon[r.category] || '⚙️';
                const catLabel = categoryLabel[r.category] || r.category;
                const typeBadge = r.type === 'exact'
                    ? `<span class="badge badge-info">${I18n.t('desensitize.exactMatch')}</span>`
                    : `<span class="badge badge-unknown">${I18n.t('desensitize.regexMatch')}</span>`;
                const levelBadge = r.level === 'strict'
                    ? '<span class="badge badge-warning">strict</span>'
                    : '<span class="badge badge-healthy">standard</span>';
                const enabledClass = r.enabled ? 'rule-enabled' : 'rule-disabled';

                html += `
                <div class="rule-card ${enabledClass}" data-id="${r.id}">
                    <div class="rule-header">
                        <span class="rule-icon">${icon}</span>
                        <span class="rule-name">${this.escHtml(r.name)}</span>
                        ${typeBadge}
                        ${levelBadge}
                        <span class="rule-category">${this.escHtml(catLabel)}</span>
                        <label class="toggle-switch" title="${r.enabled ? I18n.t("desensitize.clickDisable") : I18n.t("desensitize.clickEnable")}">
                            <input type="checkbox" ${r.enabled ? 'checked' : ''} onchange="desensitizePage.toggleEnabled(${r.id}, this.checked)">
                            <span class="toggle-slider"></span>
                        </label>
                    </div>
                    <div class="rule-pattern"><code>${this.escHtml(r.pattern)}</code></div>
                    <div class="rule-actions">
                        <button class="btn-sm" onclick="desensitizePage.editRule(${r.id})">✏️ ${I18n.t('common.edit')}</button>
                        <button class="btn-sm btn-danger" onclick="desensitizePage.deleteRule(${r.id})">🗑️ ${I18n.t('common.delete')}</button>
                    </div>
                </div>`;
            }
        }
        container.innerHTML = html;
    }

    renderBuiltin() {
        const container = document.getElementById('desensitize-builtin');
        if (!container) return;

        const categoryIcon = {
            'IDCARD': '🪪', 'BANKCARD': '💳', 'APIKEY': '🔑', 'EMAIL': '✉️', 'PHONE': '📱', 'IP': '🌐',
        };

        let html = '';
        for (const b of this.builtin) {
            const icon = categoryIcon[b.category] || '⚙️';
            html += `
            <div class="builtin-card">
                <div class="builtin-header">
                    <span class="builtin-icon">${icon}</span>
                    <span class="builtin-name">${this.escHtml(b.name)}</span>
                    <span class="badge badge-unknown">${b.category}</span>
                </div>
                <div class="builtin-pattern"><code>${this.escHtml(b.pattern)}</code></div>
            </div>`;
        }
        container.innerHTML = html;
    }

    showAddForm() {
        this.editingId = null;
        document.getElementById('rule-form-title').textContent = I18n.t("desensitize.addFormTitle");
        document.getElementById('rule-form').reset();
        document.getElementById('rule-type').value = 'regex';
        document.getElementById('rule-category').value = 'CUSTOM';
        document.getElementById('rule-level').value = 'standard';
        document.getElementById('rule-enabled').checked = true;
        document.getElementById('rule-sort-order').value = '0';
        document.getElementById('rule-form-modal').style.display = 'flex';

        const form = document.getElementById('rule-form');
        form.onsubmit = (e) => { e.preventDefault(); this.submitForm(); };
    }

    async editRule(id) {
        const r = this.rules.find(x => x.id === id);
        if (!r) return;

        this.editingId = id;
        document.getElementById('rule-form-title').textContent = I18n.t("desensitize.editFormTitle");
        document.getElementById('rule-type').value = r.type;
        document.getElementById('rule-name').value = r.name;
        document.getElementById('rule-pattern').value = r.pattern;
        document.getElementById('rule-category').value = r.category;
        document.getElementById('rule-level').value = r.level;
        document.getElementById('rule-enabled').checked = r.enabled;
        document.getElementById('rule-sort-order').value = r.sort_order || 0;
        document.getElementById('rule-form-modal').style.display = 'flex';

        const form = document.getElementById('rule-form');
        form.onsubmit = (e) => { e.preventDefault(); this.submitForm(); };
    }

    hideForm() {
        document.getElementById('rule-form-modal').style.display = 'none';
        this.editingId = null;
    }

    async submitForm() {
        const data = {
            name: document.getElementById('rule-name').value.trim(),
            type: document.getElementById('rule-type').value,
            pattern: document.getElementById('rule-pattern').value.trim(),
            category: document.getElementById('rule-category').value,
            level: document.getElementById('rule-level').value,
            enabled: document.getElementById('rule-enabled').checked,
            sort_order: parseInt(document.getElementById('rule-sort-order').value) || 0,
        };

        if (!data.name) { showToast(I18n.t("alert.ruleNameRequiredError")); return; }
        if (!data.pattern) { showToast(I18n.t("desensitize.patternRequired")); return; }

        try {
            if (this.editingId) {
                await API.put(`/desensitize/${this.editingId}`, data);
            } else {
                await API.post('/desensitize/', data);
            }
            this.hideForm();
            await this.loadRules();
            showToast(this.editingId ? I18n.t("desensitize.updateSuccess") : I18n.t("desensitize.createSuccess"));
        } catch (e) {
            showToast(I18n.t("common.saveFailed") + (e.message || I18n.t("common.unknownError")));
        }
    }

    async toggleEnabled(id, enabled) {
        try {
            await API.put(`/desensitize/${id}`, { enabled });
            await this.loadRules();
            showToast(enabled ? I18n.t("desensitize.ruleEnabled") : I18n.t("desensitize.ruleDisabled"));
        } catch (e) {
            showToast(I18n.t("desensitize.updateFailed"));
            await this.loadRules();
        }
    }

    async deleteRule(id) {
        if (!confirm(I18n.t("desensitize.confirmDelete"))) return;
        try {
            await API.del(`/desensitize/${id}`);
            await this.loadRules();
            showToast(I18n.t("desensitize.ruleDeleted"));
        } catch (e) {
            showToast(I18n.t("common.deleteFailed") + (e.message || I18n.t("common.unknownError")));
        }
    }

    // --- 规则测试 ---
    showTestPanel() {
        document.getElementById('test-panel-modal').style.display = 'flex';
    }

    hideTestPanel() {
        document.getElementById('test-panel-modal').style.display = 'none';
    }

    async runTest() {
        const text = document.getElementById('test-text').value;
        if (!text) { showToast(I18n.t("desensitize.enterTestText")); return; }

        // 收集当前自定义规则作为测试参数
        const rules = this.rules.filter(r => r.enabled).map(r => ({
            type: r.type,
            pattern: r.pattern,
            category: r.category,
            level: r.level,
        }));

        try {
            const result = await API.post('/desensitize/test', { text, rules });
            this.renderTestResult(result);
        } catch (e) {
            showToast(I18n.t("desensitize.testFailed") + (e.message || I18n.t("common.unknownError")));
        }
    }

    renderTestResult(result) {
        const container = document.getElementById('test-result');
        if (!container) return;

        if (result.total_matches === 0) {
            container.innerHTML = `<div class="empty-state"><p>${I18n.t('desensitize.noMatch')}</p></div>`;
            return;
        }

        let html = `<div class="test-summary">${I18n.t('desensitize.matchCount')} <strong>${result.total_matches}</strong> ${I18n.t('desensitize.matchUnit')}</div>`;

        // 脱敏后结果预览
        if (result.sanitized_text) {
            html += `<div class="sanitized-preview"><strong>${I18n.t('desensitize.sanitizedResult')}</strong><code>${this.escHtml(result.sanitized_text)}</code></div>`;
        }

        for (const r of result.results) {
            const hasError = r.error;
            html += `
            <div class="test-match-card ${hasError ? 'test-error' : ''}">
                <div class="test-match-header">
                    <span class="badge badge-info">${this.escHtml(r.category)}</span>
                    ${r.is_builtin ? `<span class="badge badge-healthy">${I18n.t('desensitize.builtin')}</span>` : ''}
                    <code>${this.escHtml(r.pattern)}</code>
                    <span class="test-match-count">${r.count || 0} ${I18n.t('desensitize.matchUnit')}</span>
                </div>
                ${hasError ? `<div class="test-error-msg">${this.escHtml(r.error)}</div>` : ''}
                ${r.matches ? `<div class="test-matches">${r.matches.map(m => `<span class="match-tag">${this.escHtml(String(m))}</span>`).join('')}</div>` : ''}
            </div>`;
        }
        container.innerHTML = html;
    }

    escHtml(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
}

const desensitizePage = new DesensitizePage();
