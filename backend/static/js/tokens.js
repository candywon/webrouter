/**
 * Token 令牌管理页面 JS
 */
class TokensPage {
    constructor() {
        this.tokens = [];
        this.editingId = null;
    }

    async load() {
        await this.loadTokens();
        this.bindEvents();
    }

    async loadTokens() {
        try {
            const data = await API.get('/tokens/');
            this.tokens = data.tokens || [];
            this.render();
        } catch (e) {
            console.error('Failed to load tokens:', e);
            document.getElementById('tokens-page-content').innerHTML =
                '<div class="error-msg">加载失败，请刷新重试</div>';
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
                <h2>🔑 令牌管理</h2>
                <button class="btn-primary" onclick="tokensPage.showAddForm()">+ 创建令牌</button>
            </div>
            <div class="token-list">
        `;

        if (this.tokens.length === 0) {
            html += `
                <div class="empty-state">
                    <p>还没有创建任何令牌</p>
                    <p class="hint">点击"创建令牌"开始管理 API 访问密钥</p>
                </div>
            `;
        } else {
            for (const t of this.tokens) {
                const statusHtml = this.renderStatus(t);
                const quotaHtml = this.renderQuotaBar(t);
                const desensitizeFlag = t.desensitize_enabled
                    ? '<span class="token-flag flag-desensitize">🛡 脱敏</span>'
                    : '';
                const smartFlag = t.smart_downgrade
                    ? '<span class="token-flag flag-smart">⚡ 智能降级</span>'
                    : '';
                const expiresStr = t.expires_at
                    ? `<span class="token-expire">${t.is_expired ? '已过期' : '到期: ' + formatDate(t.expires_at)}</span>`
                    : '<span class="token-expire">永不过期</span>';

                html += `
                <div class="token-card ${!t.enabled ? 'token-disabled' : ''} ${t.is_expired ? 'token-expired' : ''}" data-id="${t.id}">
                    <div class="token-header">
                        <span class="token-name">${this.escHtml(t.name)}</span>
                        ${statusHtml}
                    </div>
                    <div class="token-meta">
                        <span class="token-key-prefix" onclick="tokensPage.copyPrefix('${this.escHtml(t.key_prefix)}')" title="点击复制">${this.escHtml(t.key_prefix)}</span>
                        <span class="token-user">用户: ${this.escHtml(t.user_id || '-')}</span>
                        ${expiresStr}
                    </div>
                    <div class="token-flags">
                        ${desensitizeFlag}
                        ${smartFlag}
                        ${t.rate_limit_rpm > 0 ? `<span class="token-flag flag-ratelimit">⏱ ${t.rate_limit_rpm} RPM</span>` : ''}
                        ${t.subnet_whitelist && t.subnet_whitelist.length > 0 ? `<span class="token-flag flag-subnet">🌐 白名单</span>` : ''}
                    </div>
                    ${quotaHtml}
                    <div class="token-models">
                        ${(t.models || []).length > 0
                            ? t.models.map(m => `<span class="model-tag">${this.escHtml(m)}</span>`).join('')
                            : '<span class="model-tag model-all">全部模型</span>'}
                    </div>
                    <div class="token-actions">
                        <button class="btn-sm" onclick="tokensPage.viewDetail(${t.id})">📊 详情</button>
                        <button class="btn-sm" onclick="tokensPage.editToken(${t.id})">✏️ 编辑</button>
                        <button class="btn-sm" onclick="tokensPage.showResetQuota(${t.id})">🔄 重置配额</button>
                        <button class="btn-sm btn-danger" onclick="tokensPage.deleteToken(${t.id})">🗑️ 删除</button>
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
                        <h3 id="token-form-title">创建令牌</h3>
                        <button class="modal-close" onclick="tokensPage.hideForm()">&times;</button>
                    </div>
                    <div class="modal-body">
                        <form id="token-form">
                            <div class="form-group">
                                <label>名称 *</label>
                                <input type="text" id="tf-name" required placeholder="如: 生产环境令牌">
                            </div>
                            <div class="form-group">
                                <label>用户 ID</label>
                                <input type="text" id="tf-user-id" placeholder="如: user-001">
                            </div>
                            <div class="form-group">
                                <label>允许模型 (逗号分隔，留空表示全部)</label>
                                <input type="text" id="tf-models" placeholder="如: gpt-4o,claude-3-opus">
                            </div>
                            <div class="form-group">
                                <label>数据源 ID (逗号分隔，留空表示全部)</label>
                                <input type="text" id="tf-provider-ids" placeholder="如: 1,2,3">
                            </div>
                            <div class="form-group">
                                <label>总额度 (元，0 表示不限)</label>
                                <input type="number" id="tf-quota-total" min="0" step="0.01" value="0" placeholder="0">
                            </div>
                            <div class="form-group">
                                <label>速率限制 RPM (0 表示不限)</label>
                                <input type="number" id="tf-rate-limit-rpm" min="0" value="0" placeholder="0">
                            </div>
                            <div class="form-group">
                                <label>子网白名单 (逗号分隔，留空表示不限)</label>
                                <input type="text" id="tf-subnet-whitelist" placeholder="如: 10.0.0.0/8,192.168.1.0/24">
                            </div>
                            <div class="form-group form-row">
                                <label class="switch-label">
                                    <input type="checkbox" id="tf-smart-downgrade">
                                    <span>智能降级</span>
                                </label>
                            </div>
                            <div class="form-group form-row">
                                <label class="switch-label">
                                    <input type="checkbox" id="tf-desensitize-enabled" onchange="tokensPage.onDesensitizeToggle()">
                                    <span>启用脱敏</span>
                                </label>
                            </div>
                            <div class="form-group" id="tf-desensitize-level-group" style="display:none">
                                <label>脱敏级别</label>
                                <select id="tf-desensitize-level">
                                    <option value="off">关闭</option>
                                    <option value="standard" selected>标准</option>
                                    <option value="strict">严格</option>
                                </select>
                            </div>
                            <div class="form-group form-row">
                                <label class="switch-label">
                                    <input type="checkbox" id="tf-enabled" checked>
                                    <span>启用令牌</span>
                                </label>
                            </div>
                            <div class="form-group">
                                <label>过期时间</label>
                                <input type="datetime-local" id="tf-expires-at">
                            </div>
                            <div class="form-actions">
                                <button type="submit" class="btn-primary">保存</button>
                                <button type="button" class="btn-secondary" onclick="tokensPage.hideForm()">取消</button>
                            </div>
                        </form>
                    </div>
                </div>
            </div>

            <!-- 显示完整 Key Modal -->
            <div id="token-key-modal" class="modal" style="display:none">
                <div class="modal-content">
                    <div class="modal-header">
                        <h3>🔑 令牌已创建</h3>
                        <button class="modal-close" onclick="tokensPage.hideKeyModal()">&times;</button>
                    </div>
                    <div class="modal-body">
                        <div class="key-warning">
                            ⚠️ 完整密钥只显示一次，请立即复制保存！关闭后无法再次查看。
                        </div>
                        <div class="key-display">
                            <code id="token-full-key"></code>
                            <button class="btn-sm" onclick="tokensPage.copyFullKey()">📋 复制</button>
                        </div>
                    </div>
                </div>
            </div>

            <!-- 重置配额 Modal -->
            <div id="token-quota-modal" class="modal" style="display:none">
                <div class="modal-content">
                    <div class="modal-header">
                        <h3>🔄 重置配额</h3>
                        <button class="modal-close" onclick="tokensPage.hideQuotaModal()">&times;</button>
                    </div>
                    <div class="modal-body">
                        <div class="form-group">
                            <label>新总额度 (元，0 表示不限)</label>
                            <input type="number" id="tq-new-total" min="0" step="0.01" value="0" placeholder="0">
                        </div>
                        <div class="form-actions">
                            <button type="button" class="btn-primary" onclick="tokensPage.submitResetQuota()">确认重置</button>
                            <button type="button" class="btn-secondary" onclick="tokensPage.hideQuotaModal()">取消</button>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Token 详情 Modal -->
            <div id="token-detail-modal" class="modal" style="display:none">
                <div class="modal-content modal-wide">
                    <div class="modal-header">
                        <h3 id="td-title">令牌详情</h3>
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
            return '<span class="token-status status-expired">⏰ 已过期</span>';
        }
        if (!t.enabled) {
            return '<span class="token-status status-disabled">⏸ 已禁用</span>';
        }
        return '<span class="token-status status-enabled">● 启用中</span>';
    }

    renderQuotaBar(t) {
        if (!t.quota_total || t.quota_total <= 0) {
            return `
                <div class="token-quota">
                    <div class="quota-info">
                        <span class="quota-label">额度</span>
                        <span class="quota-text">已用 ${formatYuan(t.quota_used)} / 不限</span>
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
                    <span class="quota-label">额度</span>
                    <span class="quota-text">已用 ${formatYuan(t.quota_used)} / ${formatYuan(t.quota_total)} (剩余 ${remaining})</span>
                </div>
                <div class="quota-bar">
                    <div class="quota-bar-fill" style="width:${pct}%;background:${barColor}"></div>
                </div>
            </div>
        `;
    }

    // ======== 表单相关 ========

    showAddForm() {
        this.editingId = null;
        document.getElementById('token-form-title').textContent = '创建令牌';
        document.getElementById('token-form').reset();
        document.getElementById('tf-enabled').checked = true;
        document.getElementById('tf-desensitize-level-group').style.display = 'none';
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
        document.getElementById('token-form-title').textContent = '编辑令牌';
        document.getElementById('tf-name').value = t.name || '';
        document.getElementById('tf-user-id').value = t.user_id || '';
        document.getElementById('tf-models').value = (t.models || []).join(', ');
        document.getElementById('tf-provider-ids').value = (t.provider_ids || []).join(', ');

        // quota_total 存储为分，输入框为元
        const quotaYuan = t.quota_total > 0 ? (t.quota_total / 100) : 0;
        document.getElementById('tf-quota-total').value = quotaYuan;
        document.getElementById('tf-rate-limit-rpm').value = t.rate_limit_rpm || 0;
        document.getElementById('tf-subnet-whitelist').value = (t.subnet_whitelist || []).join(', ');
        document.getElementById('tf-smart-downgrade').checked = !!t.smart_downgrade;
        document.getElementById('tf-desensitize-enabled').checked = !!t.desensitize_enabled;
        document.getElementById('tf-desensitize-level').value = t.desensitize_level || 'standard';
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

        const modelsStr = document.getElementById('tf-models').value.trim();
        const models = modelsStr ? modelsStr.split(',').map(s => s.trim()).filter(Boolean) : [];

        const providerIdsStr = document.getElementById('tf-provider-ids').value.trim();
        const providerIds = providerIdsStr
            ? providerIdsStr.split(',').map(s => parseInt(s.trim(), 10)).filter(n => !isNaN(n))
            : [];

        const subnetStr = document.getElementById('tf-subnet-whitelist').value.trim();
        const subnetWhitelist = subnetStr ? subnetStr.split(',').map(s => s.trim()).filter(Boolean) : [];

        const desensitizeEnabled = document.getElementById('tf-desensitize-enabled').checked;
        const desensitizeLevel = desensitizeEnabled
            ? document.getElementById('tf-desensitize-level').value
            : 'off';

        const data = {
            name: document.getElementById('tf-name').value.trim(),
            user_id: document.getElementById('tf-user-id').value.trim() || null,
            models,
            provider_ids: providerIds,
            quota_total: quotaTotalCents,
            rate_limit_rpm: parseInt(document.getElementById('tf-rate-limit-rpm').value, 10) || 0,
            subnet_whitelist: subnetWhitelist,
            smart_downgrade: document.getElementById('tf-smart-downgrade').checked,
            desensitize_enabled: desensitizeEnabled,
            desensitize_level: desensitizeLevel,
            enabled: document.getElementById('tf-enabled').checked,
            expires_at: document.getElementById('tf-expires-at').value || null,
        };

        try {
            if (this.editingId) {
                await API.put(`/tokens/${this.editingId}`, data);
                this.hideForm();
                showToast('令牌已更新');
                await this.loadTokens();
            } else {
                const result = await API.post('/tokens/', data);
                this.hideForm();
                await this.loadTokens();
                // 创建成功后显示完整 key
                if (result.key) {
                    this.showKeyModal(result.key);
                } else {
                    showToast('令牌创建成功');
                }
            }
        } catch (e) {
            alert('保存失败: ' + (e.message || '未知错误'));
        }
    }

    // ======== 显示完整 Key ========

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
    }

    copyPrefix(prefix) {
        copyToClipboard(prefix);
    }

    // ======== 删除 ========

    async deleteToken(id) {
        const t = this.tokens.find(x => x.id === id);
        if (!t) return;
        if (!confirm(`确定删除令牌 "${t.name}" 吗？此操作不可恢复。`)) return;

        try {
            await API.del(`/tokens/${id}`);
            showToast('令牌已删除');
            await this.loadTokens();
        } catch (e) {
            alert('删除失败: ' + (e.message || '未知错误'));
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
            showToast('配额已重置');
            await this.loadTokens();
        } catch (e) {
            alert('重置配额失败: ' + (e.message || '未知错误'));
        }
    }

    // ======== 详情 ========

    async viewDetail(id) {
        try {
            const t = await API.get(`/tokens/${id}`);
            this.renderDetail(t);
        } catch (e) {
            alert('加载详情失败: ' + (e.message || '未知错误'));
        }
    }

    renderDetail(t) {
        document.getElementById('td-title').textContent = `令牌详情 - ${this.escHtml(t.name)}`;

        const quotaHtml = this.renderQuotaBar(t);
        const statusHtml = this.renderStatus(t);

        let usageHtml = '';
        if (t.usage_summary) {
            const us = t.usage_summary;
            usageHtml = `
                <div class="detail-section">
                    <h4>用量摘要</h4>
                    <table class="detail-table">
                        <tr><td>总请求数</td><td>${formatNumber(us.total_requests || 0)}</td></tr>
                        <tr><td>总 Token 数</td><td>${formatNumber(us.total_tokens || 0)}</td></tr>
                        <tr><td>总费用</td><td>${formatYuan(us.total_cost_cents || 0)}</td></tr>
                    </table>
                </div>
            `;
        }

        const body = `
            <div class="detail-section">
                <table class="detail-table">
                    <tr><td style="width:140px">名称</td><td>${this.escHtml(t.name)}</td></tr>
                    <tr><td>Key 前缀</td><td><code>${this.escHtml(t.key_prefix)}</code></td></tr>
                    <tr><td>用户 ID</td><td>${this.escHtml(t.user_id || '-')}</td></tr>
                    <tr><td>状态</td><td>${statusHtml}</td></tr>
                    <tr><td>启用</td><td>${t.enabled ? '✅ 是' : '❌ 否'}</td></tr>
                    <tr><td>已过期</td><td>${t.is_expired ? '⚠️ 是' : '否'}</td></tr>
                    <tr><td>过期时间</td><td>${t.expires_at ? formatDate(t.expires_at) : '永不过期'}</td></tr>
                    <tr><td>允许模型</td><td>${(t.models || []).length > 0 ? t.models.map(m => `<span class="model-tag">${this.escHtml(m)}</span>`).join(' ') : '全部'}</td></tr>
                    <tr><td>数据源</td><td>${(t.provider_ids || []).length > 0 ? t.provider_ids.join(', ') : '全部'}</td></tr>
                    <tr><td>速率限制</td><td>${t.rate_limit_rpm > 0 ? t.rate_limit_rpm + ' RPM' : '不限'}</td></tr>
                    <tr><td>子网白名单</td><td>${(t.subnet_whitelist || []).length > 0 ? t.subnet_whitelist.join(', ') : '不限'}</td></tr>
                    <tr><td>智能降级</td><td>${t.smart_downgrade ? '✅ 开启' : '关闭'}</td></tr>
                    <tr><td>脱敏</td><td>${t.desensitize_enabled ? `✅ 开启 (${t.desensitize_level || 'standard'})` : '关闭'}</td></tr>
                    <tr><td>创建时间</td><td>${formatDate(t.created_at)}</td></tr>
                    <tr><td>更新时间</td><td>${formatDate(t.updated_at)}</td></tr>
                </table>
            </div>
            <div class="detail-section">
                <h4>额度信息</h4>
                ${quotaHtml}
            </div>
            ${usageHtml}
            <div class="detail-actions">
                <button class="btn-sm" onclick="tokensPage.loadUsage(${t.id})">📈 用量明细</button>
                <button class="btn-sm" onclick="tokensPage.loadCost(${t.id})">💰 成本明细</button>
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
            alert('加载用量明细失败: ' + (e.message || '未知错误'));
        }
    }

    renderUsageDetail(data) {
        const extra = document.getElementById('td-extra');
        if (!extra) return;

        let html = '<div class="detail-section"><h4>近7天用量 (By Model)</h4>';

        if (data.by_model && Object.keys(data.by_model).length > 0) {
            html += '<table class="detail-table"><tr><th>模型</th><th>请求数</th><th>Token 数</th></tr>';
            for (const [model, info] of Object.entries(data.by_model)) {
                html += `<tr>
                    <td>${this.escHtml(model)}</td>
                    <td>${formatNumber(info.requests || 0)}</td>
                    <td>${formatNumber(info.tokens || 0)}</td>
                </tr>`;
            }
            html += '</table>';
        } else {
            html += '<p class="text-muted">暂无用量数据</p>';
        }

        if (data.daily && data.daily.length > 0) {
            html += '<h4 style="margin-top:12px">每日用量</h4>';
            html += '<table class="detail-table"><tr><th>日期</th><th>请求数</th><th>Token 数</th></tr>';
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
            alert('加载成本明细失败: ' + (e.message || '未知错误'));
        }
    }

    renderCostDetail(data) {
        const extra = document.getElementById('td-extra');
        if (!extra) return;

        let html = '<div class="detail-section"><h4>近30天成本 (By Model)</h4>';

        if (data.by_model && Object.keys(data.by_model).length > 0) {
            html += '<table class="detail-table"><tr><th>模型</th><th>费用</th></tr>';
            for (const [model, info] of Object.entries(data.by_model)) {
                const costCents = info.cost_cents || info.cost || 0;
                html += `<tr>
                    <td>${this.escHtml(model)}</td>
                    <td>${formatYuan(costCents)}</td>
                </tr>`;
            }
            html += '</table>';
        } else {
            html += '<p class="text-muted">暂无成本数据</p>';
        }

        if (data.daily && data.daily.length > 0) {
            html += '<h4 style="margin-top:12px">每日成本</h4>';
            html += '<table class="detail-table"><tr><th>日期</th><th>费用</th></tr>';
            for (const d of data.daily) {
                const costCents = d.cost_cents || d.cost || 0;
                html += `<tr>
                    <td>${this.escHtml(d.date || d.day || '-')}</td>
                    <td>${formatYuan(costCents)}</td>
                </tr>`;
            }
            html += '</table>';
        }

        html += '</div>';
        extra.innerHTML = html;
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
