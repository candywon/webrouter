// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/**
 * Token 令牌管理页面 JS
 */
class TokensPage {
    constructor() {
        this.tokens = [];
        this.editingId = null;
        this.providers = [];
        this.allModels = []; // 所有可用模型
    }

    async load() {
        await this.loadProviders();
        await this.loadTokens();
        this.bindEvents();
    }

    async loadProviders() {
        try {
            const data = await API.get('/providers/');
            this.providers = data.providers || [];
            // 收集所有模型
            const modelSet = new Set();
            for (const p of this.providers) {
                if (p.models && Array.isArray(p.models)) {
                    p.models.forEach(m => modelSet.add(m));
                }
            }
            this.allModels = Array.from(modelSet).sort();
        } catch (e) {
            console.warn('Failed to load providers for token form:', e);
        }
    }

    async loadTokens() {
        try {
            const data = await API.get('/tokens/');
            this.tokens = data.tokens || [];
            this.render();
        } catch (e) {
            console.error('Failed to load tokens:', e);
            document.getElementById('tokens-page-content').innerHTML =
                '<div class="error-msg">' + I18n.t('common.loadFailedRetry') + '</div>';
        }
    }

    bindEvents() {
        // 事件由 render() 中的 onclick 内联处理
    }

    render() {
        const container = document.getElementById('tokens-page-content');
        if (!container) return;

        let html = `
            <div class="page-header">
                <h2>${I18n.t('tokens.title')}</h2>
                <button class="btn-primary" onclick="tokensPage.showAddForm()">${I18n.t('tokens.createToken')}</button>
            </div>
            <div class="token-list">
        `;

        if (this.tokens.length === 0) {
            html += `
                <div class="empty-state">
                    <p>${I18n.t('tokens.noTokens')}</p>
                    <p class="hint">${I18n.t('tokens.createTokenHint')}</p>
                </div>
            `;
        } else {
            for (const t of this.tokens) {
                const statusHtml = this.renderStatus(t);
                const quotaHtml = this.renderQuotaBar(t);
                const desensitizeFlag = t.desensitize_enabled
                    ? `<span class="token-flag flag-desensitize">${I18n.t('tokens.desensitizeFlag')}</span>`
                    : '';
                const smartFlag = t.smart_downgrade
                    ? `<span class="token-flag flag-smart">${I18n.t('tokens.smartDowngradeFlag')}</span>`
                    : '';
                const expiresStr = t.expires_at
                    ? `<span class="token-expire">${t.is_expired ? I18n.t('common.expired') : I18n.t('tokens.expiresAt') + formatDate(t.expires_at)}</span>`
                    : `<span class="token-expire">${I18n.t('common.neverExpires')}</span>`;

                html += `
                <div class="token-card ${!t.enabled ? 'token-disabled' : ''} ${t.is_expired ? 'token-expired' : ''}" data-id="${t.id}">
                    <div class="token-header">
                        <span class="token-name">${this.escHtml(t.name)}</span>
                        ${statusHtml}
                    </div>
                    <div class="token-meta">
                        <span class="token-key-prefix" onclick="tokensPage.copyTokenInfo(${t.id})" title="${I18n.t('tokens.copyInfoTitle')}">${this.escHtml(t.key_prefix)}</span>
                        <span class="token-org">${t.org_name ? '📁 ' + this.escHtml(t.org_name) : '<span style="color:var(--text-muted)">' + I18n.t('tokens.unassignedOrg') + '</span>'}</span>
                        ${t.member_email ? `<span class="token-email">${this.escHtml(t.member_email)}</span>` : ''}
                        ${expiresStr}
                    </div>
                    <div class="token-flags">
                        ${desensitizeFlag}
                        ${smartFlag}
                        ${t.rate_limit_rpm > 0 ? `<span class="token-flag flag-ratelimit">⏱ ${t.rate_limit_rpm} RPM</span>` : ''}
                        ${t.subnet_whitelist && t.subnet_whitelist.length > 0 ? `<span class="token-flag flag-subnet">${I18n.t('tokens.whitelistFlag')}</span>` : ''}
                    </div>
                    ${quotaHtml}
                    <div class="token-models">
                        ${(t.models || []).length > 0
                            ? t.models.map(m => `<span class="model-tag">${this.escHtml(m)}</span>`).join('')
                            : `<span class="model-tag model-all">${I18n.t('common.allModels')}</span>`}
                    </div>
                    <div class="token-actions">
                        <button class="btn-sm" onclick="tokensPage.showKey(${t.id})">${I18n.t('tokens.showKey')}</button>
                        <button class="btn-sm" onclick="tokensPage.viewDetail(${t.id})">${I18n.t('tokens.viewDetail')}</button>
                        <button class="btn-sm" onclick="tokensPage.editToken(${t.id})">${I18n.t('common.edit')}</button>
                        <button class="btn-sm" onclick="tokensPage.showResetQuota(${t.id})">${I18n.t('tokens.resetQuota')}</button>
                        <button class="btn-sm btn-danger" onclick="tokensPage.deleteToken(${t.id})">${I18n.t('common.delete')}</button>
                    </div>
                </div>
                `;
            }
        }

        html += `
            </div>

            <!-- 创建/编辑表单 Modal -->
            <div id="token-form-modal" class="modal" style="display:none">
                <div class="modal-content">
                    <div class="modal-header">
                        <h3 id="token-form-title">${I18n.t('tokens.createFormTitle')}</h3>
                        <button class="modal-close" onclick="tokensPage.hideForm()">&times;</button>
                    </div>
                    <div class="modal-body">
                        <form id="token-form">
                            <div class="form-group">
                                <label>${I18n.t('common.nameRequired')}</label>
                                <input type="text" id="tf-name" required placeholder="${I18n.t('tokens.namePlaceholder')}">
                            </div>
                            <div class="form-group">
                                <label>${I18n.t('tokens.org')}</label>
                                <select id="tf-org-id">
                                    <option value="">${I18n.t('common.unassigned')}</option>
                                </select>
                            </div>
                            <div class="form-group">
                                <label>${I18n.t('tokens.allowedModels')}</label>
                                <div id="tf-models-select" class="multi-select"></div>
                            </div>
                            <div class="form-group">
                                <label>${I18n.t('common.provider')}</label>
                                <div id="tf-provider-select" class="multi-select"></div>
                            </div>
                            <div class="form-group">
                                <label>${I18n.t('tokens.quotaTotalHint')}</label>
                                <input type="number" id="tf-quota-total" min="0" step="0.01" value="0" placeholder="0">
                            </div>
                            <div class="form-group">
                                <label>${I18n.t('tokens.rateLimitHint')}</label>
                                <input type="number" id="tf-rate-limit-rpm" min="0" value="0" placeholder="0">
                            </div>
                            <div class="form-group">
                                <label>${I18n.t('tokens.subnetWhitelistHint')}</label>
                                <input type="text" id="tf-subnet-whitelist" placeholder="${I18n.t('tokens.subnetPlaceholder')}">
                            </div>
                            <div class="form-group form-row">
                                <label class="switch-label">
                                    <input type="checkbox" id="tf-smart-downgrade">
                                    <span>${I18n.t('tokens.smartDowngrade')}</span>
                                </label>
                            </div>
                            <div class="form-group form-row">
                                <label class="switch-label">
                                    <input type="checkbox" id="tf-desensitize-enabled" onchange="tokensPage.onDesensitizeToggle()">
                                    <span>${I18n.t('tokens.enableDesensitize')}</span>
                                </label>
                            </div>
                            <div class="form-group" id="tf-desensitize-level-group" style="display:none">
                                <label>${I18n.t('tokens.desensitizeLevel')}</label>
                                <select id="tf-desensitize-level">
                                    <option value="off">${I18n.t('common.close')}</option>
                                    <option value="standard" selected>${I18n.t('common.standard')}</option>
                                    <option value="strict">${I18n.t('common.strict')}</option>
                                </select>
                            </div>
                            <div class="form-group form-row">
                                <label class="switch-label">
                                    <input type="checkbox" id="tf-session-recall-enabled" checked>
                                    <span>${I18n.t('tokens.sessionRecall')}</span>
                                </label>
                            </div>
                            <div class="form-group form-row">
                                <label class="switch-label">
                                    <input type="checkbox" id="tf-enabled" checked>
                                    <span>${I18n.t('tokens.enableToken')}</span>
                                </label>
                            </div>
                            <div class="form-group">
                                <label>${I18n.t('tokens.expiresTime')}</label>
                                <input type="datetime-local" id="tf-expires-at">
                            </div>
                            <div class="form-actions">
                                <button type="submit" class="btn-primary">${I18n.t('common.save')}</button>
                                <button type="button" class="btn-secondary" onclick="tokensPage.hideForm()">${I18n.t('common.cancel')}</button>
                            </div>
                        </form>
                    </div>
                </div>
            </div>

            <!-- 显示完整 Key Modal -->
            <div id="token-key-modal" class="modal" style="display:none">
                <div class="modal-content">
                    <div class="modal-header">
                        <h3>${I18n.t('tokens.keyCreated')}</h3>
                        <button class="modal-close" onclick="tokensPage.hideKeyModal()">&times;</button>
                    </div>
                    <div class="modal-body">
                        <div class="key-warning">
                            ${I18n.t('tokens.keyWarning')}
                        </div>
                        <div class="key-display">
                            <code id="token-full-key"></code>
                            <button class="btn-sm" onclick="tokensPage.copyFullKey()">${I18n.t('common.copy')}</button>
                        </div>
                    </div>
                </div>
            </div>

            <!-- 重置配额 Modal -->
            <div id="token-quota-modal" class="modal" style="display:none">
                <div class="modal-content">
                    <div class="modal-header">
                        <h3>${I18n.t('tokens.resetQuotaTitle')}</h3>
                        <button class="modal-close" onclick="tokensPage.hideQuotaModal()">&times;</button>
                    </div>
                    <div class="modal-body">
                        <div class="form-group">
                            <label>${I18n.t('tokens.newQuotaHint')}</label>
                            <input type="number" id="tq-new-total" min="0" step="0.01" value="0" placeholder="0">
                        </div>
                        <div class="form-actions">
                            <button type="button" class="btn-primary" onclick="tokensPage.submitResetQuota()">${I18n.t('common.confirmReset')}</button>
                            <button type="button" class="btn-secondary" onclick="tokensPage.hideQuotaModal()">${I18n.t('common.cancel')}</button>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Token 详情 Modal -->
            <div id="token-detail-modal" class="modal" style="display:none">
                <div class="modal-content modal-wide">
                    <div class="modal-header">
                        <h3 id="td-title">${I18n.t('tokens.detailTitle')}</h3>
                        <button class="modal-close" onclick="tokensPage.hideDetailModal()">&times;</button>
                    </div>
                    <div class="modal-body" id="td-body">
                    </div>
                </div>
            </div>
        `;

        container.innerHTML = html;
    }

    // ======== 状态渲染 ========

    renderStatus(t) {
        if (t.is_expired) {
            return `<span class="token-status status-expired">${I18n.t('tokens.expiredStatus')}</span>`;
        }
        if (!t.enabled) {
            return `<span class="token-status status-disabled">${I18n.t('tokens.disabledStatus')}</span>`;
        }
        return `<span class="token-status status-enabled">${I18n.t('tokens.enabledStatus')}</span>`;
    }

    renderQuotaBar(t) {
        if (!t.quota_total || t.quota_total <= 0) {
            return `
                <div class="token-quota">
                    <div class="quota-info">
                        <span class="quota-label">${I18n.t('tokens.quota')}</span>
                        <span class="quota-text">${I18n.t('tokens.usedSlashUnlimited', {used: formatYuan(t.quota_used)})}</span>
                    </div>
                </div>
            `;
        }

        const ratio = t.quota_ratio != null ? t.quota_ratio : (t.quota_used / t.quota_total);
        const pct = Math.min(Math.max(ratio * 100, 0), 100);
        let barColor = 'var(--success)';
        if (ratio >= 0.9) barColor = 'var(--danger)';
        else if (ratio >= 0.7) barColor = 'var(--warning)';

        const remaining = t.quota_remaining != null && t.quota_remaining >= 0
            ? formatYuan(t.quota_remaining)
            : '-';

        return `
            <div class="token-quota">
                <div class="quota-info">
                    <span class="quota-label">${I18n.t('tokens.quota')}</span>
                    <span class="quota-text">${I18n.t('tokens.usedSlashTotal', {used: formatYuan(t.quota_used), total: formatYuan(t.quota_total), remaining: remaining})}</span>
                </div>
                <div class="quota-bar">
                    <div class="quota-bar-fill" style="width:${pct}%;background:${barColor}"></div>
                </div>
            </div>
        `;
    }

    // ======== 表单相关 ========

    async loadOrgsForForm(selectedOrgId) {
        try {
            const data = await API.get('/team/orgs');
            const orgs = data.orgs || [];
            const sel = document.getElementById('tf-org-id');
            sel.innerHTML = `<option value="">${I18n.t('common.unassigned')}</option>`;
            for (const o of orgs) {
                const indent = o.parent_id ? '└ ' : '';
                sel.innerHTML += `<option value="${o.id}" ${selectedOrgId === o.id ? 'selected' : ''}>${indent}${this.escHtml(o.name)}</option>`;
            }
        } catch (e) {
            console.error('Failed to load orgs:', e);
        }
    }

    async showAddForm() {
        this.editingId = null;
        document.getElementById('token-form-title').textContent = I18n.t("tokens.createFormTitle");
        document.getElementById('token-form').reset();
        document.getElementById('tf-enabled').checked = true;
        document.getElementById('tf-desensitize-level-group').style.display = 'none';
        this.renderMultiSelects([], []); // 创建时默认全部
        await this.loadOrgsForForm();
        document.getElementById('token-form-modal').style.display = 'flex';

        const form = document.getElementById('token-form');
        form.onsubmit = (e) => {
            e.preventDefault();
            this.submitForm();
        };
    }

    async editToken(id) {
        const t = this.tokens.find(x => x.id === id);
        if (!t) return;

        this.editingId = id;
        document.getElementById('token-form-title').textContent = I18n.t("tokens.editFormTitle");
        document.getElementById('tf-name').value = t.name || '';
        await this.loadOrgsForForm(t.org_id || null);

        // quota_total 存储为分，输入框为元
        const quotaYuan = t.quota_total > 0 ? (t.quota_total / 100) : 0;
        document.getElementById('tf-quota-total').value = quotaYuan;
        document.getElementById('tf-rate-limit-rpm').value = t.rate_limit_rpm || 0;
        document.getElementById('tf-subnet-whitelist').value = (t.subnet_whitelist || []).join(', ');
        document.getElementById('tf-smart-downgrade').checked = !!t.smart_downgrade;
        document.getElementById('tf-desensitize-enabled').checked = !!t.desensitize_enabled;
        document.getElementById('tf-desensitize-level').value = t.desensitize_level || 'standard';
        document.getElementById('tf-session-recall-enabled').checked = !!t.session_recall_enabled;
        document.getElementById('tf-enabled').checked = t.enabled !== false;
        this.onDesensitizeToggle();

        // 过期时间
        if (t.expires_at) {
            const d = new Date(t.expires_at);
            const localIso = d.toISOString().slice(0, 16);
            document.getElementById('tf-expires-at').value = localIso;
        } else {
            document.getElementById('tf-expires-at').value = '';
        }

        // 多选框：空数组 = 全部
        const models = (t.models && t.models.length > 0) ? t.models : [];
        const providerIds = (t.provider_ids && t.provider_ids.length > 0) ? t.provider_ids : [];
        this.renderMultiSelects(models, providerIds);

        document.getElementById('token-form-modal').style.display = 'flex';

        const form = document.getElementById('token-form');
        form.onsubmit = (e) => {
            e.preventDefault();
            this.submitForm();
        };
    }

    hideForm() {
        document.getElementById('token-form-modal').style.display = 'none';
        this.editingId = null;
    }

    onDesensitizeToggle() {
        const enabled = document.getElementById('tf-desensitize-enabled').checked;
        const levelGroup = document.getElementById('tf-desensitize-level-group');
        if (enabled) {
            levelGroup.style.display = '';
        } else {
            levelGroup.style.display = 'none';
            document.getElementById('tf-desensitize-level').value = 'off';
        }
    }

    async submitForm() {
        const quotaYuan = parseFloat(document.getElementById('tf-quota-total').value) || 0;
        const quotaTotalCents = Math.round(quotaYuan * 100);

        const models = this.getSelectedModels();
        const providerIds = this.getSelectedProviderIds();

        const subnetStr = document.getElementById('tf-subnet-whitelist').value.trim();
        const subnetWhitelist = subnetStr ? subnetStr.split(',').map(s => s.trim()).filter(Boolean) : [];

        const desensitizeEnabled = document.getElementById('tf-desensitize-enabled').checked;
        const desensitizeLevel = desensitizeEnabled
            ? document.getElementById('tf-desensitize-level').value
            : 'off';

        const data = {
            name: document.getElementById('tf-name').value.trim(),
            org_id: document.getElementById('tf-org-id').value ? parseInt(document.getElementById('tf-org-id').value) : null,
            models,
            provider_ids: providerIds,
            quota_total: quotaTotalCents,
            rate_limit_rpm: parseInt(document.getElementById('tf-rate-limit-rpm').value, 10) || 0,
            subnet_whitelist: subnetWhitelist,
            smart_downgrade: document.getElementById('tf-smart-downgrade').checked,
            desensitize_enabled: desensitizeEnabled,
            desensitize_level: desensitizeLevel,
            session_recall_enabled: document.getElementById('tf-session-recall-enabled').checked,
            enabled: document.getElementById('tf-enabled').checked,
            expires_at: document.getElementById('tf-expires-at').value || null,
        };

        try {
            if (this.editingId) {
                await API.put(`/tokens/${this.editingId}`, data);
                this.hideForm();
                showToast(I18n.t("tokens.updated"));
                await this.loadTokens();
            } else {
                const result = await API.post('/tokens/', data);
                this.hideForm();
                await this.loadTokens();
                // 创建成功后显示完整 key
                if (result.key) {
                    this.showKeyModal(result.key);
                } else {
                    showToast(I18n.t("tokens.created"));
                }
            }
        } catch (e) {
            alert(I18n.t("common.saveFailed") + (e.message || I18n.t("common.unknownError")));
        }
    }

    // ======== 显示完整 Key ========

    async showKey(id) {
        try {
            const data = await API.get(`/tokens/${id}/key`);
            if (data.key) {
                this.showKeyModal(data.key);
            } else {
                showToast(I18n.t("tokens.cannotGetKey"));
            }
        } catch (e) {
            showToast(I18n.t("tokens.getKeyFailed") + (e.message || I18n.t("common.unknownError")));
        }
    }

    showKeyModal(fullKey) {
        document.getElementById('token-full-key').textContent = fullKey;
        document.getElementById('token-key-modal').style.display = 'flex';
    }

    hideKeyModal() {
        document.getElementById('token-key-modal').style.display = 'none';
    }

    copyFullKey() {
        const key = document.getElementById('token-full-key').textContent;
        copyToClipboard(key);
        showToast(I18n.t("tokens.keyCopied"));
    }

    copyPrefix(prefix) {
        copyToClipboard(prefix);
    }

    copyTokenInfo(id) {
        const t = this.tokens.find(x => x.id === id);
        if (!t) return;
        const quotaTotal = t.quota_total > 0 ? formatYuan(t.quota_total) : I18n.t("common.unlimited");
        const quotaUsed = formatYuan(t.quota_used || 0);
        const quotaRemain = t.quota_remaining != null && t.quota_remaining >= 0
            ? formatYuan(t.quota_remaining)
            : '-';
        const models = (t.models && t.models.length > 0) ? t.models.join(', ') : I18n.t("common.allModels");
        const status = t.enabled ? (t.is_expired ? I18n.t("common.expired") : I18n.t("common.enable")) : I18n.t("common.disabled");
        const expiresAt = t.expires_at ? formatDate(t.expires_at) : I18n.t("common.neverExpires");
        const desensitize = t.desensitize_enabled ? I18n.t("tokens.desensitizeOn") + (t.desensitize_level || I18n.t("common.default")) + ')' : I18n.t("common.off");
        const smartDowngrade = t.smart_downgrade ? I18n.t("common.on") : I18n.t("common.off");
        const rpm = t.rate_limit_rpm > 0 ? t.rate_limit_rpm + ' RPM' : I18n.t("common.unlimited");
        const subnet = (t.subnet_whitelist && t.subnet_whitelist.length > 0)
            ? t.subnet_whitelist.join(', ')
            : I18n.t("common.unlimited");

        const info = [
            `${I18n.t('tokens.tokenName')}: ${t.name}`,
            `${I18n.t('tokens.keyPrefix')}: ${t.key_prefix}`,
            `${I18n.t('common.status')}: ${status}`,
            `${I18n.t('tokens.expiresTime')}: ${expiresAt}`,
            `${I18n.t('tokens.quotaTotal')}: ${quotaTotal}`,
            `${I18n.t('tokens.quotaUsed')}: ${quotaUsed}`,
            `${I18n.t('tokens.quotaRemaining')}: ${quotaRemain}`,
            `${I18n.t('tokens.rateLimit')}: ${rpm}`,
            `${I18n.t('tokens.ipWhitelist')}: ${subnet}`,
            `${I18n.t('tokens.availableModels')}: ${models}`,
            `${I18n.t('tokens.desensitizeRule')}: ${desensitize}`,
            `${I18n.t('tokens.smartDowngrade')}: ${smartDowngrade}`,
        ].join('\n');

        copyToClipboard(info);
        showToast(I18n.t("tokens.infoCopied"));
    }

    // ======== 删除 ========

    async deleteToken(id) {
        const t = this.tokens.find(x => x.id === id);
        if (!t) return;
        if (!confirm(I18n.t('tokens.confirmDelete', {name: t.name}))) return;

        try {
            await API.del(`/tokens/${id}`);
            showToast(I18n.t("tokens.deleted"));
            await this.loadTokens();
        } catch (e) {
            alert(I18n.t("common.deleteFailed") + (e.message || I18n.t("common.unknownError")));
        }
    }

    // ======== 重置配额 ========

    showResetQuota(id) {
        const t = this.tokens.find(x => x.id === id);
        if (!t) return;
        this._resetQuotaId = id;
        const currentYuan = t.quota_total > 0 ? (t.quota_total / 100) : 0;
        document.getElementById('tq-new-total').value = currentYuan;
        document.getElementById('token-quota-modal').style.display = 'flex';
    }

    hideQuotaModal() {
        document.getElementById('token-quota-modal').style.display = 'none';
        this._resetQuotaId = null;
    }

    async submitResetQuota() {
        if (!this._resetQuotaId) return;
        const newTotalYuan = parseFloat(document.getElementById('tq-new-total').value) || 0;
        const newTotalCents = Math.round(newTotalYuan * 100);

        try {
            await API.post(`/tokens/${this._resetQuotaId}/reset-quota`, {
                quota_total: newTotalCents,
            });
            this.hideQuotaModal();
            showToast(I18n.t("tokens.quotaReset"));
            await this.loadTokens();
        } catch (e) {
            alert(I18n.t("tokens.resetQuotaFailed") + (e.message || I18n.t("common.unknownError")));
        }
    }

    // ======== 详情 ========

    async viewDetail(id) {
        try {
            const t = await API.get(`/tokens/${id}`);
            this.renderDetail(t);
        } catch (e) {
            alert(I18n.t("tokens.loadDetailFailed") + (e.message || I18n.t("common.unknownError")));
        }
    }

    renderDetail(t) {
        document.getElementById('td-title').textContent = I18n.t('tokens.detailTitleWith') + this.escHtml(t.name);

        const quotaHtml = this.renderQuotaBar(t);
        const statusHtml = this.renderStatus(t);

        let usageHtml = '';
        if (t.usage_summary) {
            const us = t.usage_summary;
            usageHtml = `
                <div class="detail-section">
                    <h4>${I18n.t('tokens.usageSummary')}</h4>
                    <table class="detail-table">
                        <tr><td>${I18n.t('common.totalRequests')}</td><td>${formatNumber(us.total_requests || 0)}</td></tr>
                        <tr><td>${I18n.t('tokens.totalTokens')}</td><td>${formatNumber(us.total_tokens || 0)}</td></tr>
                        <tr><td>${I18n.t('common.totalCost')}</td><td>${formatYuan(us.total_cost_cents || 0)}</td></tr>
                    </table>
                </div>
            `;
        }

        const body = `
            <div class="detail-section">
                <table class="detail-table">
                    <tr><td style="width:140px">${I18n.t('common.name')}</td><td>${this.escHtml(t.name)}</td></tr>
                    <tr><td>${I18n.t('tokens.keyPrefix')}</td><td><code>${this.escHtml(t.key_prefix)}</code></td></tr>
                    <tr><td>${I18n.t('tokens.org')}</td><td>${t.org_name ? this.escHtml(t.org_name) : I18n.t('common.notAssigned')}</td></tr>
                    <tr><td>${I18n.t('tokens.memberEmail')}</td><td>${this.escHtml(t.member_email || '-')}</td></tr>
                    <tr><td>${I18n.t('common.status')}</td><td>${statusHtml}</td></tr>
                    <tr><td>${I18n.t('common.enable')}</td><td>${t.enabled ? I18n.t("common.yes") : I18n.t("common.noEmoji")}</td></tr>
                    <tr><td>${I18n.t('common.expired')}</td><td>${t.is_expired ? I18n.t("common.warningYes") : I18n.t("common.no")}</td></tr>
                    <tr><td>${I18n.t('tokens.expiresTime')}</td><td>${t.expires_at ? formatDate(t.expires_at) : I18n.t('common.neverExpires')}</td></tr>
                    <tr><td>${I18n.t('tokens.allowedModels')}</td><td>${(t.models || []).length > 0 ? t.models.map(m => `<span class="model-tag">${this.escHtml(m)}</span>`).join(' ') : I18n.t('common.all')}</td></tr>
                    <tr><td>${I18n.t('common.provider')}</td><td>${(t.provider_ids || []).length > 0 ? t.provider_ids.join(', ') : I18n.t('common.all')}</td></tr>
                    <tr><td>${I18n.t('tokens.rateLimit')}</td><td>${t.rate_limit_rpm > 0 ? t.rate_limit_rpm + ' RPM' : I18n.t('common.unlimited')}</td></tr>
                    <tr><td>${I18n.t('tokens.subnetWhitelist')}</td><td>${(t.subnet_whitelist || []).length > 0 ? t.subnet_whitelist.join(', ') : I18n.t('common.unlimited')}</td></tr>
                    <tr><td>${I18n.t('tokens.smartDowngrade')}</td><td>${t.smart_downgrade ? I18n.t("common.enabledYes") : I18n.t("common.off")}</td></tr>
                    <tr><td>${I18n.t('tokens.desensitize')}</td><td>${t.desensitize_enabled ? (() => { const labels = {off: I18n.t('common.close'), standard: I18n.t('common.standard'), strict: I18n.t('common.strict')}; return I18n.t('tokens.desensitizeEnabledWith', {level: labels[t.desensitize_level] || I18n.t("common.standard")}); })() : I18n.t('common.close')}</td></tr>
                    <tr><td>${I18n.t('common.createdAt')}</td><td>${formatDate(t.created_at)}</td></tr>
                    <tr><td>${I18n.t('common.updatedAt')}</td><td>${formatDate(t.updated_at)}</td></tr>
                </table>
            </div>
            <div class="detail-section">
                <h4>${I18n.t('tokens.quotaInfo')}</h4>
                ${quotaHtml}
            </div>
            ${usageHtml}
            <div class="detail-actions">
                <button class="btn-sm" onclick="tokensPage.copyTokenInfo(${t.id})">${I18n.t('tokens.copyInfo')}</button>
                <button class="btn-sm" onclick="tokensPage.loadUsage(${t.id})">${I18n.t('tokens.usageDetail')}</button>
                <button class="btn-sm" onclick="tokensPage.loadCost(${t.id})">${I18n.t('tokens.costDetail')}</button>
            </div>
            <div id="td-extra"></div>
        `;

        document.getElementById('td-body').innerHTML = body;
        document.getElementById('token-detail-modal').style.display = 'flex';
    }

    hideDetailModal() {
        document.getElementById('token-detail-modal').style.display = 'none';
    }

    async loadUsage(id) {
        try {
            const data = await API.get(`/tokens/${id}/usage?hours=168`);
            this.renderUsageDetail(data);
        } catch (e) {
            alert(I18n.t("tokens.loadUsageFailed") + (e.message || I18n.t("common.unknownError")));
        }
    }

    renderUsageDetail(data) {
        const extra = document.getElementById('td-extra');
        if (!extra) return;

        let html = `<div class="detail-section"><h4>${I18n.t('tokens.recent7Days')}</h4>`;

        if (data.by_model && Object.keys(data.by_model).length > 0) {
            html += `<table class="detail-table"><tr><th>${I18n.t('common.model')}</th><th>${I18n.t('common.requestsCount')}</th><th>${I18n.t('tokens.tokenCount')}</th></tr>`;
            for (const [model, info] of Object.entries(data.by_model)) {
                html += `<tr>
                    <td>${this.escHtml(model)}</td>
                    <td>${formatNumber(info.requests || 0)}</td>
                    <td>${formatNumber(info.tokens || 0)}</td>
                </tr>`;
            }
            html += '</table>';
        } else {
            html += `<p class="text-muted">${I18n.t('common.noUsageData')}</p>`;
        }

        if (data.daily && data.daily.length > 0) {
            html += `<h4 style="margin-top:12px">${I18n.t('tokens.dailyUsage')}</h4>`;
            html += `<table class="detail-table"><tr><th>${I18n.t('common.date')}</th><th>${I18n.t('common.requestsCount')}</th><th>${I18n.t('tokens.tokenCount')}</th></tr>`;
            for (const d of data.daily) {
                html += `<tr>
                    <td>${this.escHtml(d.date || d.day || '-')}</td>
                    <td>${formatNumber(d.requests || 0)}</td>
                    <td>${formatNumber(d.tokens || 0)}</td>
                </tr>`;
            }
            html += '</table>';
        }

        html += '</div>';
        extra.innerHTML = html;
    }

    async loadCost(id) {
        try {
            const data = await API.get(`/tokens/${id}/cost?days=30`);
            this.renderCostDetail(data);
        } catch (e) {
            alert(I18n.t('common.loadCostFailed') + ': ' + (e.message || I18n.t("common.unknownError")));
        }
    }

    renderCostDetail(data) {
        const extra = document.getElementById('td-extra');
        if (!extra) return;

        let html = `<div class="detail-section"><h4>${I18n.t('tokens.recent30Days')}</h4>`;

        if (data.by_model && Object.keys(data.by_model).length > 0) {
            html += `<table class="detail-table"><tr><th>${I18n.t('common.model')}</th><th>${I18n.t('common.costColumn')}</th></tr>`;
            for (const [model, info] of Object.entries(data.by_model)) {
                const costCents = info.cost_cents || info.cost || 0;
                html += `<tr>
                    <td>${this.escHtml(model)}</td>
                    <td>${formatYuan(costCents)}</td>
                </tr>`;
            }
            html += '</table>';
        } else {
            html += `<p class="text-muted">${I18n.t('tokens.noCostData')}</p>`;
        }

        if (data.daily && data.daily.length > 0) {
            html += `<h4 style="margin-top:12px">${I18n.t('tokens.dailyCost')}</h4>`;
            html += `<table class="detail-table"><tr><th>${I18n.t('common.date')}</th><th>${I18n.t('common.costColumn')}</th></tr>`;
            for (const d of data.daily) {
                const costCents = d.cost_cents || d.cost || 0;
                html += `<tr>
                    <td>${this.escHtml(d.date || d.day || '-')}</td>
                    <td>${formatYuan(costCents)}</td>
                </tr>`;
            }
            html += '</table>';
        }

        html += `<p class="text-muted" style="margin-top:12px;font-size:12px;color:var(--text-secondary);">${I18n.t('tokens.costDisclaimer')}</p>`;
        html += '</div>';
        extra.innerHTML = html;
    }

    // ======== 多选组件 ========

    /**
     * 渲染模型和 Provider 多选下拉框
     * @param {string[]} selectedModels - 已选模型（空=全部）
     * @param {number[]} selectedProviderIds - 已选 Provider ID（空=全部）
     */
    renderMultiSelects(selectedModels, selectedProviderIds) {
        this._renderModelSelect(selectedModels);
        this._renderProviderSelect(selectedProviderIds);
    }

    _renderModelSelect(selected) {
        const container = document.getElementById('tf-models-select');
        if (!container) return;
        const isAll = selected.length === 0;
        const displayText = isAll ? I18n.t("common.allModels") : selected.join(', ');

        let optionsHtml = '';
        for (const model of this.allModels) {
            const checked = isAll ? 'checked disabled' : (selected.includes(model) ? 'checked' : '');
            optionsHtml += `<label class="ms-item"><input type="checkbox" value="${this.escHtml(model)}" ${checked}><span>${this.escHtml(model)}</span></label>`;
        }

        container.innerHTML = `
            <div class="ms-display" onclick="this.parentElement.classList.toggle('ms-open')">
                <span class="ms-label">${this.escHtml(displayText)}</span>
                <span class="ms-arrow">▼</span>
            </div>
            <div class="ms-dropdown">
                <label class="ms-item ms-all">
                    <input type="checkbox" id="ms-models-all" ${isAll ? 'checked' : ''}>
                    <span>${I18n.t('common.all')}</span>
                </label>
                <div class="ms-options">${optionsHtml}</div>
            </div>
        `;

        // 绑定事件
        const allCb = container.querySelector('#ms-models-all');
        allCb.onchange = () => {
            const items = container.querySelectorAll('.ms-options input[type=checkbox]');
            items.forEach(cb => { cb.checked = allCb.checked; cb.disabled = allCb.checked; });
            this._updateModelDisplay(container);
        };

        container.querySelectorAll('.ms-options input[type=checkbox]').forEach(cb => {
            cb.onchange = () => {
                const allChecked = Array.from(container.querySelectorAll('.ms-options input[type=checkbox]')).every(c => c.checked);
                allCb.checked = allChecked;
                this._updateModelDisplay(container);
            };
        });

        // 点击外部关闭
        setTimeout(() => {
            document.addEventListener('click', (e) => {
                if (!container.contains(e.target)) container.classList.remove('ms-open');
            }, { once: false });
        }, 100);
    }

    _renderProviderSelect(selected) {
        const container = document.getElementById('tf-provider-select');
        if (!container) return;
        const isAll = selected.length === 0;
        const displayText = isAll ? I18n.t("common.allProviders") : selected.map(id => {
            const p = this.providers.find(x => x.id === id);
            return p ? p.name : `#${id}`;
        }).join(', ');

        let optionsHtml = '';
        for (const p of this.providers) {
            const checked = isAll ? 'checked disabled' : (selected.includes(p.id) ? 'checked' : '');
            optionsHtml += `<label class="ms-item"><input type="checkbox" value="${p.id}" ${checked}><span>${this.escHtml(p.name)}</span></label>`;
        }

        if (this.providers.length === 0) {
            optionsHtml = `<div style="padding:8px;color:var(--text-muted);font-size:12px">${I18n.t('tokens.noProvidersFirst')}</div>`;
        }

        container.innerHTML = `
            <div class="ms-display" onclick="this.parentElement.classList.toggle('ms-open')">
                <span class="ms-label">${this.escHtml(displayText)}</span>
                <span class="ms-arrow">▼</span>
            </div>
            <div class="ms-dropdown">
                <label class="ms-item ms-all">
                    <input type="checkbox" id="ms-providers-all" ${isAll ? 'checked' : ''}>
                    <span>${I18n.t('common.all')}</span>
                </label>
                <div class="ms-options">${optionsHtml}</div>
            </div>
        `;

        const allCb = container.querySelector('#ms-providers-all');
        if (allCb) {
            allCb.onchange = () => {
                const items = container.querySelectorAll('.ms-options input[type=checkbox]');
                items.forEach(cb => { cb.checked = allCb.checked; cb.disabled = allCb.checked; });
                this._updateProviderDisplay(container);
            };
        }

        container.querySelectorAll('.ms-options input[type=checkbox]').forEach(cb => {
            cb.onchange = () => {
                const allChecked = Array.from(container.querySelectorAll('.ms-options input[type=checkbox]')).every(c => c.checked);
                allCb.checked = allChecked;
                this._updateProviderDisplay(container);
            };
        });

        setTimeout(() => {
            document.addEventListener('click', (e) => {
                if (!container.contains(e.target)) container.classList.remove('ms-open');
            }, { once: false });
        }, 100);
    }

    _updateModelDisplay(container) {
        const cbs = container.querySelectorAll('.ms-options input[type=checkbox]');
        const checked = Array.from(cbs).filter(cb => cb.checked).map(cb => cb.value);
        const isAll = container.querySelector('#ms-models-all')?.checked || checked.length === 0;
        const label = container.querySelector('.ms-label');
        if (label) label.textContent = isAll ? I18n.t("common.allModels") : checked.join(', ');
    }

    _updateProviderDisplay(container) {
        const cbs = container.querySelectorAll('.ms-options input[type=checkbox]');
        const checkedIds = Array.from(cbs).filter(cb => cb.checked).map(cb => parseInt(cb.value, 10));
        const isAll = container.querySelector('#ms-providers-all')?.checked || checkedIds.length === 0;
        const label = container.querySelector('.ms-label');
        if (label) {
            label.textContent = isAll ? I18n.t("common.allProviders") : checkedIds.map(id => {
                const p = this.providers.find(x => x.id === id);
                return p ? p.name : `#${id}`;
            }).join(', ');
        }
    }

    getSelectedModels() {
        const container = document.getElementById('tf-models-select');
        if (!container) return [];
        const isAll = container.querySelector('#ms-models-all')?.checked || false;
        if (isAll) return [];
        const cbs = container.querySelectorAll('.ms-options input[type=checkbox]');
        return Array.from(cbs).filter(cb => cb.checked).map(cb => cb.value);
    }

    getSelectedProviderIds() {
        const container = document.getElementById('tf-provider-select');
        if (!container) return [];
        const isAll = container.querySelector('#ms-providers-all')?.checked || false;
        if (isAll) return [];
        const cbs = container.querySelectorAll('.ms-options input[type=checkbox]');
        return Array.from(cbs).filter(cb => cb.checked).map(cb => parseInt(cb.value, 10));
    }

    // ======== 工具方法 ========

    escHtml(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
}

// 全局实例
const tokensPage = new TokensPage();
