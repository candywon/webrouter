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
                    </div>
                    <div class="provider-meta">
                        <span class="provider-url">${this.escHtml(p.base_url)}</span>
                        <span class="provider-latency">${latency}</span>
                        <span class="provider-checked">${I18n.t('providers.checkedAt')}${checked}</span>
                    </div>
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
                            <div class="form-group">
                                <label>Base URL *</label>
                                <input type="text" id="pf-base-url" required placeholder="e.g. https://api.openai.com">
                                <button type="button" class="btn-sm" onclick="providersPage.autoDetect()" style="margin-top:4px">🔍 ${I18n.t('providers.autoDetect')}</button>
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

    showAddForm() {
        this.editingId = null;
        document.getElementById('form-title').textContent = I18n.t("providers.addFormTitle");
        document.getElementById('provider-form').reset();
        document.getElementById('pf-type').value = 'direct';
        document.getElementById('pf-models').value = '';
        document.getElementById('pf-fallback-enabled').checked = true;
        this.onTypeChange();
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
        document.getElementById('pf-notes').value = p.notes || '';
        const models = (p.models && Array.isArray(p.models)) ? p.models.join(', ') : (p.models || '');
        document.getElementById('pf-models').value = models;
        document.getElementById('pf-fallback-enabled').checked = p.fallback_enabled !== false;
        this.onTypeChange();
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
            notes: document.getElementById('pf-notes').value.trim(),
            fallback_enabled: document.getElementById('pf-fallback-enabled').checked,
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
            if (data.detected_type) {
                document.getElementById('pf-type').value = data.detected_type;
                this.onTypeChange();
                alert(I18n.t('providers.detectedType') + (data.type_config?.label || data.detected_type));
            }
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
