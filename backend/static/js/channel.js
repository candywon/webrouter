// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/**
 * Provider Channel 渠道管理页面 JS
 * 嵌套在 Provider 详情内，管理同一 Provider 下的多渠道（多 Key）
 */
class ChannelPage {
    constructor() {
        this.channels = [];
        this.provider = null;
        this.defaultChannel = null;
        this.providerId = null;
    }

    async load() {
        // 从 URL hash 提取 provider_id
        const hash = window.location.hash;
        const m = hash.match(/#\/providers\/(\d+)\/channels/);
        if (!m) {
            document.getElementById('page-content').innerHTML = '<div class="empty-state"><p>' + I18n.t('channel.enterFromProvider') + '</p></div>';
            return;
        }
        this.providerId = parseInt(m[1]);
        await this.loadChannels();
    }

    async loadChannels() {
        try {
            const data = await API.get(`/providers/${this.providerId}/channels`);
            this.provider = data.provider;
            this.defaultChannel = data.default_channel;
            this.channels = data.channels || [];
            this.render();
        } catch (e) {
            console.error('Failed to load channels:', e);
            document.getElementById('page-content').innerHTML =
                '<div class="empty-state"><p>' + I18n.t('channel.loadFailed') + '</p></div>';
        }
    }

    render() {
        const container = document.getElementById('page-content-channels');
        if (!container) return;

        const pName = this.provider ? this.provider.name : '';
        let html = `
            <div class="page-header">
                <h2>📡 ${this.escHtml(pName)} — ${I18n.t('channel.title')}</h2>
                <div>
                    <button class="btn-secondary" onclick="channelPage.goBack()">← ${I18n.t('channel.backToProviders')}</button>
                    <button class="btn-primary" onclick="channelPage.showAddForm()">+ ${I18n.t('channel.addChannel')}</button>
                    <button class="btn-secondary" onclick="channelPage.showBatchForm()">${I18n.t('channel.batchAdd')}</button>
                </div>
            </div>`;

        // 默认渠道（Provider 自身配置）
        if (this.defaultChannel) {
            const dc = this.defaultChannel;
            html += `
            <div class="card">
                <div class="card-header">
                    <span class="card-title">🏠 ${I18n.t('channel.defaultChannel')}</span>
                </div>
                <div class="channel-default-info">
                    <table>
                        <tr><td style="width:120px;color:var(--text-secondary)">Base URL</td><td>${this.escHtml(dc.base_url || '-')}</td></tr>
                        <tr><td style="color:var(--text-secondary)">API Key</td><td><code>${this.escHtml(dc.api_key_masked || '***')}</code></td></tr>
                        <tr><td style="color:var(--text-secondary)">${I18n.t('common.availableModels')}</td><td>${dc.models && dc.models.length ? dc.models.map(m => `<span class="model-tag">${this.escHtml(m)}</span>`).join('') : I18n.t('common.all')}</td></tr>
                        <tr><td style="color:var(--text-secondary)">${I18n.t('channel.priorityWeight')}</td><td>${dc.priority} / ${dc.weight}</td></tr>
                        <tr><td style="color:var(--text-secondary)">${I18n.t('common.status')}</td><td>${statusBadge(dc.status)}</td></tr>
                    </table>
                </div>
            </div>`;
        }

        // 自定义渠道列表
        html += `<div class="card"><div class="card-header"><span class="card-title">${I18n.t('channel.channelList')} (${this.channels.length})</span></div>`;

        if (this.channels.length === 0) {
            html += '<div class="empty-state"><p>' + I18n.t('channel.noChannels') + '</p><p class="hint">' + I18n.t('channel.channelHint') + '</p></div>';
        } else {
            for (const ch of this.channels) {
                const enabled = ch.enabled !== false;
                html += `
                <div class="channel-card ${enabled ? '' : 'channel-disabled'}">
                    <div class="channel-header">
                        <span class="channel-name">${this.escHtml(ch.name)}</span>
                        ${enabled ? '<span class="badge badge-healthy">${I18n.t("common.enabledStatus")}</span>' : '<span class="badge badge-unknown">${I18n.t("common.disabledStatus")}</span>'}
                        <span class="channel-id">#${ch.id}</span>
                    </div>
                    <div class="channel-meta">
                        ${ch.resolved_base_url ? `<span title="Base URL">🌐 ${this.escHtml(ch.resolved_base_url)}</span>` : ''}
                        ${ch.resolved_priority != null ? `<span title="${I18n.t('common.priority')}">⬆ ${ch.resolved_priority}</span>` : ''}
                        ${ch.resolved_weight != null ? `<span title="${I18n.t('common.weight')}">⚖ ${ch.resolved_weight}</span>` : ''}
                        ${ch.rate_limit_rpm ? `<span title="RPM">⏱ ${ch.rate_limit_rpm}/min</span>` : ''}
                    </div>
                    ${ch.resolved_models && ch.resolved_models.length ? `<div class="channel-models">${ch.resolved_models.map(m => `<span class="model-tag">${this.escHtml(m)}</span>`).join('')}</div>` : ''}
                    ${ch.notes ? `<div class="channel-notes">${this.escHtml(ch.notes)}</div>` : ''}
                    <div class="channel-actions">
                        <button class="btn-sm" onclick="channelPage.editChannel(${ch.id})">✏️ ${I18n.t('common.edit')}</button>
                        <button class="btn-sm btn-danger" onclick="channelPage.deleteChannel(${ch.id})">🗑️ ${I18n.t('common.delete')}</button>
                    </div>
                </div>`;
            }
        }
        html += '</div>';

        // 渠道表单
        html += this._formHTML();
        // 批量添加表单
        html += this._batchFormHTML();

        container.innerHTML = html;
    }

    _formHTML() {
        return `
        <div id="channel-form-modal" class="modal" style="display:none">
            <div class="modal-content">
                <div class="modal-header">
                    <h3 id="channel-form-title">${I18n.t('channel.addChannel')}</h3>
                    <button class="modal-close" onclick="channelPage.hideForm()">&times;</button>
                </div>
                <div class="modal-body">
                    <form id="channel-form">
                        <div class="form-group">
                            <label>${I18n.t('channel.nameRequired')}</label>
                            <input type="text" id="cf-name" required placeholder="${I18n.t('channel.namePlaceholder')}">
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('channel.baseUrlInherit')}</label>
                            <input type="text" id="cf-base-url" placeholder="https://...">
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('channel.apiKeyInherit')}</label>
                            <input type="password" id="cf-api-key" placeholder="sk-xxx">
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('common.modelsHint')}</label>
                            <input type="text" id="cf-models" placeholder="gpt-4o, claude-3.5-sonnet">
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('common.priority')}</label>
                            <input type="number" id="cf-priority" value="0" min="0" max="100">
                            <span class="hint">${I18n.t('channel.priorityHint')}</span>
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('common.weight')}</label>
                            <input type="number" id="cf-weight" value="0" min="0" max="100">
                            <span class="hint">${I18n.t('channel.weightHint')}</span>
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('channel.rateLimitHint')}</label>
                            <input type="number" id="cf-rate-limit" value="0" min="0">
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('channel.costMultiplierHint')}</label>
                            <input type="number" id="cf-cost-mult" value="0" step="0.1" min="0">
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('common.notes')}</label>
                            <textarea id="cf-notes" rows="2" placeholder="${I18n.t('common.placeholderOptional')}"></textarea>
                        </div>
                        <div class="form-group">
                            <label><input type="checkbox" id="cf-enabled" checked> ${I18n.t("common.enabledStatus")}</label>
                        </div>
                        <div class="form-actions">
                            <button type="submit" class="btn-primary">${I18n.t('common.save')}</button>
                            <button type="button" class="btn-secondary" onclick="channelPage.hideForm()">${I18n.t('common.cancel')}</button>
                        </div>
                    </form>
                </div>
            </div>
        </div>`;
    }

    _batchFormHTML() {
        return `
        <div id="batch-channel-modal" class="modal" style="display:none">
            <div class="modal-content">
                <div class="modal-header">
                    <h3>${I18n.t('channel.batchAddTitle')}</h3>
                    <button class="modal-close" onclick="channelPage.hideBatchForm()">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="form-group">
                        <label>${I18n.t('channel.batchJsonHint')}</label>
                        <textarea id="batch-channel-json" rows="8" placeholder='[{"name":"Key-1","api_key":"sk-xxx"},{"name":"Key-2","api_key":"sk-yyy"}]'></textarea>
                    </div>
                    <div class="form-actions">
                        <button class="btn-primary" onclick="channelPage.submitBatch()">${I18n.t('channel.batchCreate')}</button>
                        <button class="btn-secondary" onclick="channelPage.hideBatchForm()">${I18n.t('common.cancel')}</button>
                    </div>
                </div>
            </div>
        </div>`;
    }

    showAddForm() {
        this.editingId = null;
        document.getElementById('channel-form-title').textContent = I18n.t("channel.addFormTitle");
        document.getElementById('channel-form').reset();
        document.getElementById('cf-enabled').checked = true;
        document.getElementById('channel-form-modal').style.display = 'flex';
        const form = document.getElementById('channel-form');
        form.onsubmit = (e) => { e.preventDefault(); this.submitForm(); };
    }

    async editChannel(id) {
        const ch = this.channels.find(x => x.id === id);
        if (!ch) return;

        this.editingId = id;
        document.getElementById('channel-form-title').textContent = I18n.t("channel.editFormTitle");
        document.getElementById('cf-name').value = ch.name || '';
        document.getElementById('cf-base-url').value = ch.base_url || '';
        document.getElementById('cf-api-key').value = ''; // 不回填key
        document.getElementById('cf-models').value = (ch.models && Array.isArray(ch.models)) ? ch.models.join(', ') : (ch.models || '');
        document.getElementById('cf-priority').value = ch.priority || 0;
        document.getElementById('cf-weight').value = ch.weight || 0;
        document.getElementById('cf-rate-limit').value = ch.rate_limit_rpm || 0;
        document.getElementById('cf-cost-mult').value = ch.cost_multiplier || 0;
        document.getElementById('cf-notes').value = ch.notes || '';
        document.getElementById('cf-enabled').checked = ch.enabled !== false;
        document.getElementById('channel-form-modal').style.display = 'flex';
        const form = document.getElementById('channel-form');
        form.onsubmit = (e) => { e.preventDefault(); this.submitForm(); };
    }

    hideForm() {
        document.getElementById('channel-form-modal').style.display = 'none';
        this.editingId = null;
    }

    async submitForm() {
        const modelsStr = document.getElementById('cf-models').value.trim();
        const models = modelsStr ? modelsStr.split(',').map(s => s.trim()).filter(Boolean) : [];

        const data = {
            name: document.getElementById('cf-name').value.trim(),
            base_url: document.getElementById('cf-base-url').value.trim(),
            api_key: document.getElementById('cf-api-key').value.trim(),
            models: models,
            priority: parseInt(document.getElementById('cf-priority').value) || 0,
            weight: parseInt(document.getElementById('cf-weight').value) || 0,
            rate_limit_rpm: parseInt(document.getElementById('cf-rate-limit').value) || 0,
            cost_multiplier: parseFloat(document.getElementById('cf-cost-mult').value) || 0,
            notes: document.getElementById('cf-notes').value.trim(),
            enabled: document.getElementById('cf-enabled').checked,
        };

        if (!data.name) { showToast(I18n.t("channel.nameRequiredError")); return; }

        try {
            if (this.editingId) {
                await API.put(`/providers/${this.providerId}/channels/${this.editingId}`, data);
            } else {
                await API.post(`/providers/${this.providerId}/channels`, data);
            }
            this.hideForm();
            await this.loadChannels();
            showToast(I18n.t("channel.saveSuccess"));
        } catch (e) {
            showToast(I18n.t('common.saveFailed') + ': ' + (e.message || I18n.t("common.unknownError")));
        }
    }

    async deleteChannel(id) {
        const ch = this.channels.find(x => x.id === id);
        if (!ch) return;
        if (!confirm(I18n.t('channel.confirmDelete', {name: ch.name}))) return;

        try {
            await API.del(`/providers/${this.providerId}/channels/${id}`);
            await this.loadChannels();
            showToast(I18n.t("channel.deleted"));
        } catch (e) {
            showToast(I18n.t('common.deleteFailed') + ': ' + (e.message || I18n.t("common.unknownError")));
        }
    }

    showBatchForm() {
        document.getElementById('batch-channel-modal').style.display = 'flex';
    }

    hideBatchForm() {
        document.getElementById('batch-channel-modal').style.display = 'none';
    }

    async submitBatch() {
        const jsonStr = document.getElementById('batch-channel-json').value.trim();
        let channels;
        try {
            channels = JSON.parse(jsonStr);
            if (!Array.isArray(channels)) throw new Error(I18n.t("common.arrayRequired"));
        } catch (e) {
            showToast(I18n.t('common.jsonError') + e.message);
            return;
        }

        try {
            const result = await API.post(`/providers/${this.providerId}/channels/batch`, { channels });
            this.hideBatchForm();
            await this.loadChannels();
            showToast(result.message || I18n.t("channel.batchCreateDone"));
        } catch (e) {
            showToast(I18n.t('channel.batchCreateFailed') + ' ' + (e.message || I18n.t("common.unknownError")));
        }
    }

    goBack() {
        Router.navigate('/providers');
    }

    escHtml(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
}

const channelPage = new ChannelPage();
