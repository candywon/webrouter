// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/**
 * Provider 数据源管理页面 JS
 */
class ProvidersPage {
    constructor() {
        this.providers = [];
        this.types = {};
        this.editingId = null;
    }

    async load() {
        await this.init();
    }

    async init() {
        await this.loadTypes();
        await this.loadProviders();
        this.bindEvents();
    }

    async loadTypes() {
        try {
            const data = await API.get('/providers/types');
            this.types = data.types || {};
        } catch (e) {
            console.error('Failed to load provider types:', e);
        }
    }

    async loadProviders() {
        try {
            const data = await API.get('/providers/');
            this.providers = data.providers || [];
            this.sortProviders();
            this.render();
        } catch (e) {
            console.error('Failed to load providers:', e);
            document.getElementById('page-content').innerHTML =
                '<div class="error-msg">' + I18n.t('common.loadFailedRetry') + '</div>';
        }
    }

    bindEvents() {
        // 事件绑定由 render() 中的 onclick 内联处理
    }

    sortProviders() {
        const rank = (p) => {
            if (p.status === 'healthy' && p.proxy_enabled !== false) return 0;
            if (p.status === 'healthy') return 1;
            if (p.status === 'rate_limited') return 2;
            if (p.status === 'unhealthy') return 3;
            if (p.status === 'auth_failed') return 4;
            if (p.status === 'timeout') return 5;
            if (p.status === 'dead') return 6;
            return 7; // unknown / disabled
        };
        this.providers.sort((a, b) => rank(a) - rank(b) || a.id - b.id);
    }

    render() {
        const container = document.getElementById('page-content');
        if (!container) return;

        const dotFor = (status) => `<span class="status-dot dot-${status || 'unknown'}"></span>`;

        const typeLabel = {
            'direct': I18n.t("providers.direct"),
            'aggregate': I18n.t("providers.aggregate"),
            'litellm': 'LiteLLM',
            'custom': I18n.t("common.custom"),
        };

        let html = `
            <div class="page-header">
                <h2>🔌 ${I18n.t('providers.title')}</h2>
                <button class="btn-primary" onclick="providersPage.showAddForm()">+ ${I18n.t('providers.addProvider')}</button>
            </div>
            <div class="provider-list">
        `;

        if (this.providers.length === 0) {
            html += `
                <div class="empty-state">
                    <p>${I18n.t('providers.noProviders')}</p>
                    <p class="hint">${I18n.t('providers.addProviderHint')}</p>
                </div>
            `;
        } else {
            for (const p of this.providers) {
                const icon = dotFor(p.status);
                const type = typeLabel[p.type] || p.type;
                const latency = p.last_latency_ms != null ? `${p.last_latency_ms}ms` : '-';
                const checked = p.last_check_at ? this.formatTime(p.last_check_at) : I18n.t("providers.unchecked");

                html += `
                <div class="provider-card ${p.status === 'dead' ? 'provider-dead' : ''}" data-id="${p.id}">
                    <div class="provider-header">
                        <span class="provider-icon">${icon}</span>
                        <span class="provider-name">${this.escHtml(p.name)}</span>
                        <span class="provider-type badge">${type}</span>
                        <span class="provider-status status-${p.status}">${p.status}</span>
                        ${p.proxy_enabled === false ? '<span class="badge bg-secondary">未入池</span>' : ''}
                        ${p.api_format === 'anthropic' ? '<span class="badge bg-info" title="上游为 Anthropic 协议">Anthropic 协议</span>' : ''}
                    </div>
                    <div class="provider-meta">
                        <span class="provider-url">${this.escHtml(p.base_url)}</span>
                        <span class="provider-latency">${latency}</span>
                        <span class="provider-checked">${I18n.t('providers.checkedAt')}${checked}</span>
                    </div>
                    ${p.anthropic_base_url ? `<div class="provider-meta"><span class="badge bg-info">Anthropic</span> <span class="provider-url">${this.escHtml(p.anthropic_base_url)}</span></div>` : ''}
                    ${p.api_key_masked ? `<div class="provider-key">Key: ${this.escHtml(p.api_key_masked)}</div>` : ''}
                    ${p.last_error ? `<div class="provider-error">${I18n.t('providers.error')}${this.escHtml(p.last_error)}</div>` : ''}
                    <div class="provider-actions">
                        <button class="btn-sm" onclick="providersPage.checkOne(${p.id})">🔍 ${I18n.t('common.check')}</button>
                        <button class="btn-sm" onclick="Router.navigate('/providers/${p.id}/channels')">📡 ${I18n.t('nav.channels')}</button>
                        <button class="btn-sm" onclick="providersPage.editProvider(${p.id})">✏️ ${I18n.t('common.edit')}</button>
                        <button class="btn-sm btn-danger" onclick="providersPage.deleteProvider(${p.id})">🗑️ ${I18n.t('common.delete')}</button>
                    </div>
                </div>
                `;
            }
        }

        html += `
            </div>
            <div class="provider-actions-bar">
                <button class="btn-secondary" onclick="providersPage.checkAll()">🔍 ${I18n.t('providers.checkAllBtn')}</button>
            </div>

            <!-- 添加/编辑表单（隐藏） -->
            <div id="provider-form-modal" class="modal" style="display:none">
                <div class="modal-content">
                    <div class="modal-header">
                        <h3 id="form-title">${I18n.t('providers.addProvider')}</h3>
                        <button class="modal-close" onclick="providersPage.hideForm()">&times;</button>
                    </div>
                    <div class="modal-body">
                        <form id="provider-form">
                            <div class="form-group">
                                <label>${I18n.t('common.type')}</label>
                                <select id="pf-type" onchange="providersPage.onTypeChange()">
                                    <option value="direct">🔌 ${I18n.t('providers.direct')}</option>
                                    <option value="aggregate">🔀 ${I18n.t('providers.aggregate')}</option>
                                    <option value="litellm">🦙 ${I18n.t('providers.litellmProxy')}</option>
                                    <option value="custom">⚙️ ${I18n.t('providers.customGateway')}</option>
                                </select>
                            </div>
                            <div class="form-group">
                                <label>${I18n.t('common.nameRequired')}</label>
                                <input type="text" id="pf-name" required placeholder="${I18n.t('providers.namePlaceholder')}">
                            </div>

                            <!-- 主端点卡片：URL + 协议格式绑定 -->
                            <div class="endpoint-card" style="border:1px solid var(--border, #ccd);border-radius:6px;padding:12px;margin-bottom:12px;background:var(--card-bg, #fafbfc)">
                                <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:8px">
                                    <strong>📡 主端点（Base URL）</strong>
                                    <button type="button" class="btn-sm" onclick="providersPage.autoDetect()">🔍 ${I18n.t('providers.autoDetect')}</button>
                                </div>
                                <div class="form-group" style="margin-bottom:8px">
                                    <input type="text" id="pf-base-url" required placeholder="e.g. https://api.openai.com">
                                </div>
                                <div class="form-group" style="margin-bottom:0">
                                    <label style="font-size:0.9em;color:var(--text-secondary)">协议格式</label>
                                    <select id="pf-api-format" onchange="providersPage.onApiFormatChange()">
                                        <option value="auto">Auto-detect（按已知 vendor 规则匹配，未匹配按 OpenAI 处理）</option>
                                        <option value="openai">OpenAI 格式（/v1/chat/completions + Authorization: Bearer）</option>
                                        <option value="anthropic">Anthropic 格式（/v1/messages + x-api-key）</option>
                                    </select>
                                    <span class="hint">该格式描述上方 Base URL 这一个端点的协议；填错会导致请求失败</span>
                                </div>
                            </div>

                            <!-- 第二端点（可选）：同厂商若同时提供 OpenAI + Anthropic 两套兼容接口 -->
                            <div class="endpoint-card" style="border:1px dashed var(--border, #ccd);border-radius:6px;padding:12px;margin-bottom:12px;background:var(--card-bg-alt, #fafbfc)">
                                <div style="display:flex;align-items:center;gap:8px;margin-bottom:8px">
                                    <strong>🅰 另加一个 Anthropic URL</strong>
                                    <span class="hint" style="margin:0">（可选 — 同厂商提供双协议时使用）</span>
                                </div>
                                <div class="form-group" style="margin-bottom:0">
                                    <input type="text" id="pf-anthropic-base-url" placeholder="e.g. https://ark.cn-beijing.volces.com/api/v3/anthropic">
                                    <span class="hint">配置后 Anthropic 客户端请求会直发此端点（不翻译，保留 thinking blocks/tool_use）；OpenAI 客户端仍走主端点。<br>主端点本身就是 Anthropic 时无需填写。</span>
                                </div>
                            </div>
                            <div class="form-group" id="pf-api-key-group">
                                <label>API Key</label>
                                <input type="password" id="pf-api-key" placeholder="sk-xxx">
                            </div>
                            <div class="form-group" id="pf-master-key-group" style="display:none">
                                <label>Master Key</label>
                                <input type="password" id="pf-master-key" placeholder="LiteLLM Master Key">
                            </div>
                            <div class="form-group" id="pf-health-endpoint-group" style="display:none">
                                <label>${I18n.t('providers.healthEndpoint')}</label>
                                <input type="text" id="pf-health-endpoint" placeholder="${I18n.t('common.placeholderExampleHealth')}">
                            </div>
                            <div class="form-group">
                                <label>${I18n.t('providers.modelsHint')}</label>
                                <input type="text" id="pf-models" placeholder="${I18n.t('common.placeholderExampleModels')}">
                                <span class="hint">${I18n.t('providers.modelsUnlimitedHint')}</span>
                            </div>
                            <div class="form-group">
                                <label>${I18n.t('common.notes')}</label>
                                <textarea id="pf-notes" rows="2" placeholder="${I18n.t('common.placeholderOptional')}"></textarea>
                            </div>
                            <div class="form-group">
                                <label style="display:flex;align-items:center;gap:8px;cursor:pointer">
                                    <input type="checkbox" id="pf-proxy-enabled" checked>
                                    <span>纳入代理池</span>
                                </label>
                                <span class="hint">开启后该数据源可通过代理网关转发请求</span>
                            </div>
                            <div class="form-group">
                                <label style="display:flex;align-items:center;gap:8px;cursor:pointer">
                                    <input type="checkbox" id="pf-fallback-enabled" checked>
                                    <span>${I18n.t('providers.fallbackEnabled')}</span>
                                </label>
                                <span class="hint">${I18n.t('providers.fallbackHint')}</span>
                            </div>
                            <div class="form-actions">
                                <button type="submit" class="btn-primary">${I18n.t('common.save')}</button>
                                <button type="button" class="btn-secondary" onclick="providersPage.hideForm()">${I18n.t('common.cancel')}</button>
                            </div>
                        </form>
                    </div>
                </div>
            </div>
        `;

        container.innerHTML = html;
    }

    onTypeChange() {
        const type = document.getElementById('pf-type').value;
        document.getElementById('pf-master-key-group').style.display =
            (type === 'litellm') ? '' : 'none';
        document.getElementById('pf-health-endpoint-group').style.display =
            (type === 'custom') ? '' : 'none';
    }

    onApiFormatChange() {
        // 主端点已是 Anthropic 时，第二端点输入框淡化并提示
        const fmt = document.getElementById('pf-api-format').value;
        const anthInput = document.getElementById('pf-anthropic-base-url');
        if (!anthInput) return;
        if (fmt === 'anthropic') {
            anthInput.disabled = true;
            anthInput.placeholder = '主端点已是 Anthropic 协议，无需再填';
            anthInput.value = '';
        } else {
            anthInput.disabled = false;
            anthInput.placeholder = 'e.g. https://ark.cn-beijing.volces.com/api/v3/anthropic';
        }
    }

    showAddForm() {
        this.editingId = null;
        document.getElementById('form-title').textContent = I18n.t("providers.addFormTitle");
        document.getElementById('provider-form').reset();
        document.getElementById('pf-type').value = 'direct';
        document.getElementById('pf-models').value = '';
        document.getElementById('pf-anthropic-base-url').value = '';
        document.getElementById('pf-api-format').value = 'auto';
        document.getElementById('pf-proxy-enabled').checked = true;
        document.getElementById('pf-fallback-enabled').checked = true;
        this.onTypeChange();
        this.onApiFormatChange();
        document.getElementById('provider-form-modal').style.display = 'flex';

        const form = document.getElementById('provider-form');
        form.onsubmit = (e) => {
            e.preventDefault();
            this.submitForm();
        };
    }

    async editProvider(id) {
        const p = this.providers.find(x => x.id === id);
        if (!p) return;

        this.editingId = id;
        document.getElementById('form-title').textContent = I18n.t("providers.editFormTitle");
        document.getElementById('pf-type').value = p.type;
        document.getElementById('pf-name').value = p.name;
        document.getElementById('pf-base-url').value = p.base_url;
        document.getElementById('pf-anthropic-base-url').value = p.anthropic_base_url || '';
        document.getElementById('pf-api-format').value = p.api_format || 'auto';
        document.getElementById('pf-notes').value = p.notes || '';
        const models = (p.models && Array.isArray(p.models)) ? p.models.join(', ') : (p.models || '');
        document.getElementById('pf-models').value = models;
        document.getElementById('pf-fallback-enabled').checked = p.fallback_enabled !== false;
        document.getElementById('pf-proxy-enabled').checked = p.proxy_enabled !== false;
        this.onTypeChange();
        this.onApiFormatChange();
        document.getElementById('provider-form-modal').style.display = 'flex';

        const form = document.getElementById('provider-form');
        form.onsubmit = (e) => {
            e.preventDefault();
            this.submitForm();
        };
    }

    hideForm() {
        document.getElementById('provider-form-modal').style.display = 'none';
        this.editingId = null;
    }

    async submitForm() {
        const type = document.getElementById('pf-type').value;
        const modelsStr = document.getElementById('pf-models').value.trim();
        const models = modelsStr ? modelsStr.split(',').map(s => s.trim()).filter(Boolean) : [];
        const data = {
            type,
            name: document.getElementById('pf-name').value.trim(),
            base_url: document.getElementById('pf-base-url').value.trim(),
            anthropic_base_url: document.getElementById('pf-anthropic-base-url').value.trim(),
            api_format: document.getElementById('pf-api-format').value,
            notes: document.getElementById('pf-notes').value.trim(),
            fallback_enabled: document.getElementById('pf-fallback-enabled').checked,
            proxy_enabled: document.getElementById('pf-proxy-enabled').checked,
        };

        if (models.length > 0) data.models = models;

        const keyVal = document.getElementById('pf-api-key').value.trim();
        if (keyVal) {
            data.api_key = keyVal;
        } else if (!this.editingId) {
            showToast(I18n.t("providers.apiKeyRequired"));
            return;
        }
        // 编辑时 key 留空表示不修改

        if (type === 'litellm') {
            data.master_key = document.getElementById('pf-master-key').value.trim();
        }
        if (type === 'custom') {
            data.health_endpoint = document.getElementById('pf-health-endpoint').value.trim();
        }

        try {
            if (this.editingId) {
                await API.put(`/providers/${this.editingId}`, data);
            } else {
                await API.post('/providers/', data);
            }
            this.hideForm();
            await this.loadProviders();
        } catch (e) {
            alert(I18n.t("common.saveFailed") + (e.message || I18n.t("common.unknownError")));
        }
    }

    async checkOne(id) {
        try {
            const result = await API.post(`/providers/${id}/check`);
            alert(`${result.name}: ${result.status} (${result.latency_ms || 0}ms)`);
            await this.loadProviders();
        } catch (e) {
            alert(I18n.t("common.checkFailed") + (e.message || I18n.t("common.unknownError")));
        }
    }

    async checkAll() {
        try {
            const data = await API.post('/providers/check_all');
            alert(I18n.t('providers.checkAllDone', {total: data.total}));
            await this.loadProviders();
        } catch (e) {
            alert(I18n.t("providers.checkAllFailed") + (e.message || I18n.t("common.unknownError")));
        }
    }

    async deleteProvider(id) {
        const p = this.providers.find(x => x.id === id);
        if (!p) return;
        if (!confirm(I18n.t('providers.confirmDelete', {name: p.name}))) return;

        try {
            await API.del(`/providers/${id}`);
            await this.loadProviders();
        } catch (e) {
            alert(I18n.t("common.deleteFailed") + (e.message || I18n.t("common.unknownError")));
        }
    }

    async autoDetect() {
        const baseUrl = document.getElementById('pf-base-url').value.trim();
        if (!baseUrl) {
            alert(I18n.t("providers.enterBaseUrl"));
            return;
        }

        try {
            const data = await API.post('/providers/detect', { base_url: baseUrl });
            const msgs = [];
            if (data.detected_type) {
                document.getElementById('pf-type').value = data.detected_type;
                this.onTypeChange();
                msgs.push(I18n.t('providers.detectedType') + (data.type_config?.label || data.detected_type));
            }
            if (data.detected_anthropic_base_url) {
                const cur = document.getElementById('pf-anthropic-base-url').value.trim();
                if (!cur) {
                    document.getElementById('pf-anthropic-base-url').value = data.detected_anthropic_base_url;
                }
                msgs.push('Anthropic 端点: ' + data.detected_anthropic_base_url);
            }
            const suggested = data.suggested_api_format || data.detected_api_format;
            if (suggested && suggested !== 'auto') {
                const fmtSel = document.getElementById('pf-api-format');
                if (fmtSel.value === 'auto') {
                    fmtSel.value = suggested;
                }
                const rule = data.matched_pattern ? `（命中规则 ${data.matched_pattern}）` : '';
                msgs.push(`建议 API 格式: ${suggested}${rule}\n如不正确请在下拉框手动选择`);
            } else {
                msgs.push('API 格式: 未匹配已知 vendor，请在下拉框选择 OpenAI 或 Anthropic');
            }
            this.onApiFormatChange();
            if (msgs.length) alert(msgs.join('\n'));
        } catch (e) {
            console.warn('Auto detect failed:', e);
        }
    }

    formatTime(isoStr) {
        if (!isoStr) return '-';
        try {
            const d = new Date(isoStr);
            return d.toLocaleString('zh-CN', { hour12: false });
        } catch {
            return isoStr;
        }
    }

    escHtml(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
}

// 全局实例
const providersPage = new ProvidersPage();
