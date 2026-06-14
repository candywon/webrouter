// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/**
 * 系统设置管理页面 JS
 * 功能：查看和编辑系统设置（持久化到 wr_system_settings 表）
 */
class SettingsPage {
    constructor() {
        this.settings = [];
        this.categories = [];
        this.filterCategory = '';
        this.editingKey = null;
        this.providers = [];
        this.kb = null;
    }

    async load() {
        await this.loadProviders();
        await this.loadKbStatus();
        await this.loadSettings();
        // 自动初始化 wr-proxy 优化特性开关
        const hasFeature = this.settings.some(s => s.key === 'feature_dynamic_content_last');
        if (!hasFeature) {
            await this.seedFeatures();
        }
        // 自动初始化复杂度配置
        const hasComplexity = this.settings.some(s => s.key === 'smart_complexity_config');
        if (!hasComplexity) {
            await this.seedFeatures();
        }
    }

    async loadProviders() {
        try {
            const data = await API.get('/providers/');
            this.providers = (data.providers || []).filter(p => p.enabled);
        } catch (e) {
            this.providers = [];
        }
    }

    async loadKbStatus() {
        try {
            const res = await fetch('/api/knowledge/status');
            this.kb = await res.json();
        } catch (e) {
            this.kb = null;
        }
    }

    async loadSettings() {
        try {
            const data = await API.get('/settings/all');
            this.settings = data.settings || [];
            this.buildCategories();
            this.render();
        } catch (e) {
            console.error('Failed to load settings:', e);
            const el = document.getElementById('settings-content');
            if (el) el.innerHTML = `<div class="empty-state"><p>${I18n.t('common.loadFailedRetry')}</p></div>`;
        }
    }

    buildCategories() {
        const cats = new Set(this.settings.map(s => s.category));
        this.categories = Array.from(cats).sort();
    }

    render() {
        const container = document.getElementById('settings-content');
        if (!container) return;

        const categoryLabels = {
            'general': I18n.t("settings.general"),
            'proxy': I18n.t("settings.proxy"),
            'monitor': I18n.t("settings.monitor"),
            'alert': I18n.t("settings.alert"),
            'advanced': I18n.t("settings.advanced"),
        };

        let html = `
            <div class="page-header">
                <h2>${I18n.t('settings.title')}</h2>
                <div>
                    <button class="btn-secondary" onclick="settingsPage.loadSettings()">${I18n.t('common.refresh')}</button>
                </div>
            </div>

            ${this.renderGatewayCard()}

            ${this.renderSpecialCards()}

            <div class="card">
                <div class="card-header"><span class="card-title">${I18n.t('settings.allSettings')}</span></div>
                ${this.renderSettingsTable()}
            </div>

            <div class="card">
                <div class="card-header"><span class="card-title">${I18n.t('settings.dataBackup')}</span></div>
                <div style="display:flex;gap:12px;padding:16px;">
                    <button class="btn-primary" onclick="settingsPage.backup()">${I18n.t('settings.createBackup')}</button>
                    <button class="btn-secondary" onclick="settingsPage.showRestoreDialog()">${I18n.t('settings.restoreBackup')}</button>
                </div>
            </div>
        `;

        container.innerHTML = html;
    }

    // 渲染代理网关设置卡片（置顶显示）
    renderGatewayCard() {
        const proxyEnabled = this.settings.find(s => s.key === 'proxy_enabled');
        const proxyUrl = this.settings.find(s => s.key === 'proxy_url');
        const gatewayUrl = this.settings.find(s => s.key === 'gateway_url');
        const routingStrategy = this.settings.find(s => s.key === 'routing_strategy');
        const defaultTimeout = this.settings.find(s => s.key === 'default_timeout');
        const maxFailover = this.settings.find(s => s.key === 'max_failover');
        const maxRetryCount = this.settings.find(s => s.key === 'max_retry_count');

        const proxyOn = proxyEnabled && proxyEnabled.value;

        return `
        <div class="card" style="border-left:3px solid ${proxyOn ? 'var(--success)' : 'var(--danger)'};">
            <div class="card-header">
                <span class="card-title">${I18n.t('settings.gatewaySettings')}</span>
                <label class="toggle-switch">
                    <input type="checkbox" ${proxyOn ? 'checked' : ''}
                           onchange="settingsPage.toggleProxySwitch(this.checked)">
                    <span class="toggle-slider"></span>
                    <span class="toggle-label">${proxyOn ? I18n.t("settings.enabled") : I18n.t("settings.disabled")}</span>
                </label>
            </div>
            <div style="padding:16px;">
                <p style="color:var(--text-muted);font-size:12px;margin-bottom:12px;">${I18n.t('settings.gatewayDescription')}</p>
                <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">
                    <div class="form-group">
                        <label>${I18n.t('settings.proxyUrl')}</label>
                        <input type="text" id="gw-proxy-url" value="${this.escHtml(proxyUrl ? proxyUrl.value : '')}"
                               onchange="settingsPage.updateString('proxy_url', this.value)"
                               placeholder="http://localhost:5051">
                        <span style="font-size:11px;color:var(--text-muted);">${I18n.t('settings.proxyUrlHint')}</span>
                    </div>
                    <div class="form-group">
                        <label>${I18n.t('settings.gatewayUrl')}</label>
                        <input type="text" id="gw-gateway-url" value="${this.escHtml(gatewayUrl ? gatewayUrl.value : '')}"
                               onchange="settingsPage.updateString('gateway_url', this.value)"
                               placeholder="http://public-ip-or-domain:5051">
                        <span style="font-size:11px;color:var(--text-muted);">${I18n.t('settings.gatewayUrlHint')}</span>
                    </div>
                    <div class="form-group">
                        <label>${I18n.t('settings.routingStrategy')}</label>
                        <select onchange="settingsPage.updateString('routing_strategy', this.value)">
                            ${['smart','priority','round_robin','least_latency','cost_first'].map(s =>
                                `<option value="${s}" ${routingStrategy && routingStrategy.value === s ? 'selected' : ''}>${this.ROUTING_STRATEGY_LABELS[s]}</option>`
                            ).join('')}
                        </select>
                    </div>
                    <div class="form-group">
                        <label>${I18n.t('settings.defaultTimeout')}</label>
                        <input type="number" value="${defaultTimeout ? defaultTimeout.value : 60}" min="1"
                               onchange="settingsPage.updateNumber('default_timeout', this.value, 'int')">
                    </div>
                    <div class="form-group">
                        <label>${I18n.t('settings.maxFailover')}</label>
                        <input type="number" value="${maxFailover ? maxFailover.value : 3}" min="0" max="10"
                               onchange="settingsPage.updateNumber('max_failover', this.value, 'int')">
                    </div>
                    <div class="form-group">
                        <label>${I18n.t('settings.maxRetryCount')}</label>
                        <input type="number" value="${maxRetryCount ? maxRetryCount.value : 2}" min="0" max="10"
                               onchange="settingsPage.updateNumber('max_retry_count', this.value, 'int')">
                    </div>
                </div>
            </div>
        </div>`;
    }

    // 渲染特殊设置卡片（日志清理、厂商测试、告警通知等专用管理区域）
    renderSpecialCards() {
        let html = '';
        const logSetting = this.settings.find(s => s.key === 'log_retention_days');
        const healthSetting = this.settings.find(s => s.key === 'health_test_configs');
        const alertSetting = this.settings.find(s => s.key === 'alert_smtp_host');

        // 暂停知识库（仅 KB 已开通时显示，紧贴 wr-proxy 特性之前）
        if (this.kb && this.kb.enabled) {
            html += this.renderKbPauseCard();
        }

        // wr-proxy 优化特性开关
        html += this.renderFeatureToggles();

        // 六维度复杂度配置
        html += this.renderComplexityConfig();

        // 知识提取（LLM 评估）配置
        html += this.renderExtractConfig();

        // 日志清理周期
        if (logSetting) {
            html += `
            <div class="card">
                <div class="card-header"><span class="card-title">${I18n.t('settings.logCleanup')}</span></div>
                <div style="padding:16px;display:flex;align-items:center;gap:12px;">
                    <label style="font-size:14px;">${I18n.t('settings.logRetentionDays')}</label>
                    <input type="number" id="log-retention-input" value="${logSetting.value}" min="1" max="365"
                           style="width:80px;padding:6px 10px;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);color:var(--text-primary);font-size:14px;">
                    <span style="color:var(--text-muted);font-size:13px;">${I18n.t('settings.daysDetail', {value: logSetting.value})}</span>
                    <button class="btn-primary" onclick="settingsPage.saveLogRetention()" style="margin-left:auto;">${I18n.t('common.save')}</button>
                </div>
            </div>`;
        }

        // 厂商健康测试配置（高级，默认折叠）
        if (healthSetting) {
            const configs = Array.isArray(healthSetting.value) ? healthSetting.value : [];
            html += `
            <div class="card">
                <details>
                    <summary style="display:flex;align-items:center;padding:12px 16px;cursor:pointer;list-style:none;border-bottom:1px solid var(--border);">
                        <span class="card-title">${I18n.t('settings.healthTestConfig')}</span>
                        <span style="margin-left:8px;padding:2px 8px;background:var(--bg-card);border:1px solid var(--border);border-radius:10px;font-size:11px;color:var(--text-muted);">${I18n.t('settings.advanced')}</span>
                        <span style="margin-left:auto;font-size:12px;color:var(--text-muted);">▾</span>
                    </summary>
                    <div style="padding:16px;">
                        <p style="color:var(--text-muted);font-size:12px;margin-bottom:12px;">${I18n.t('settings.healthTestHint')}</p>
                        <div style="margin-bottom:12px;">
                            <button class="btn-primary btn-sm" onclick="settingsPage.addHealthConfig()">${I18n.t('settings.addVendor')}</button>
                        </div>
                        <table>
                            <thead><tr>
                                <th>${I18n.t('settings.vendorName')}</th><th>${I18n.t('settings.domainMatch')}</th><th>${I18n.t('settings.testEndpoint')}</th><th>${I18n.t('settings.testBody')}</th><th>${I18n.t('common.actions')}</th>
                            </tr></thead>
                            <tbody>
                                ${configs.map((cfg, i) => `
                                <tr>
                                    <td><input type="text" value="${this.escHtml(cfg.name || '')}"
                                               onchange="settingsPage.updateHealthConfig(${i}, 'name', this.value)"
                                               style="width:100px;padding:4px 8px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:12px;"></td>
                                    <td><input type="text" value="${this.escHtml(cfg.domain || '')}"
                                               onchange="settingsPage.updateHealthConfig(${i}, 'domain', this.value)"
                                               style="width:140px;padding:4px 8px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:12px;"></td>
                                    <td><input type="text" value="${this.escHtml(cfg.endpoint || '')}"
                                               onchange="settingsPage.updateHealthConfig(${i}, 'endpoint', this.value)"
                                               style="width:180px;padding:4px 8px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:12px;"></td>
                                    <td><input type="text" value="${this.escHtml(cfg.body || '')}"
                                               onchange="settingsPage.updateHealthConfig(${i}, 'body', this.value)"
                                               style="width:300px;padding:4px 8px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:11px;font-family:monospace;"></td>
                                    <td><button class="btn-icon" onclick="settingsPage.removeHealthConfig(${i})" title="${I18n.t('common.delete')}">🗑️</button></td>
                                </tr>`).join('')}
                            </tbody>
                        </table>
                        <button class="btn-primary" onclick="settingsPage.saveHealthConfigs()" style="margin-top:12px;">${I18n.t('settings.saveAll')}</button>
                        <button class="btn-secondary" onclick="settingsPage.resetHealthConfigs()" style="margin-top:12px;margin-left:8px;">${I18n.t('settings.restoreDefault')}</button>
                    </div>
                </details>
            </div>`;
        }

        // 告警通知配置
        if (alertSetting) {
            const wechatSendkey = this.settings.find(s => s.key === 'alert_wechat_sendkey')?.value || '';
            const smtpHost = this.settings.find(s => s.key === 'alert_smtp_host')?.value || '';
            const smtpPort = this.settings.find(s => s.key === 'alert_smtp_port')?.value || 587;
            const smtpUser = this.settings.find(s => s.key === 'alert_smtp_user')?.value || '';
            const smtpPass = this.settings.find(s => s.key === 'alert_smtp_password')?.value || '';
            const smtpFrom = this.settings.find(s => s.key === 'alert_smtp_from')?.value || '';
            const emailTo = this.settings.find(s => s.key === 'alert_email_to')?.value || '';

            html += `
            <div class="card">
                <div class="card-header"><span class="card-title">${I18n.t('settings.alertNotifyConfig')}</span></div>
                <div style="padding:16px;">
                    <p style="color:var(--text-muted);font-size:12px;margin-bottom:16px;">${I18n.t('settings.alertNotifyHint')}</p>
                    <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">
                        <div class="form-group" style="grid-column:span 2;">
                            <label>${I18n.t('settings.wechatSendkey')}</label>
                            <input type="text" id="alert-wechat-sendkey" value="${this.escHtml(wechatSendkey)}" placeholder="${I18n.t('settings.wechatSendkeyHint')}">
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('settings.smtpHost')}</label>
                            <input type="text" id="alert-smtp-host" value="${this.escHtml(smtpHost)}" placeholder="${I18n.t('settings.smtpHostPlaceholder')}">
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('settings.smtpPort')}</label>
                            <input type="number" id="alert-smtp-port" value="${smtpPort}" min="1" max="65535">
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('settings.smtpUser')}</label>
                            <input type="text" id="alert-smtp-user" value="${this.escHtml(smtpUser)}" placeholder="${I18n.t('settings.smtpUserPlaceholder')}">
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('settings.smtpPassword')}</label>
                            <input type="password" id="alert-smtp-password" value="${this.escHtml(smtpPass)}" placeholder="${I18n.t('settings.smtpPasswordHint')}">
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('settings.smtpFrom')}</label>
                            <input type="text" id="alert-smtp-from" value="${this.escHtml(smtpFrom)}" placeholder="${I18n.t('settings.smtpFromPlaceholder')}">
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('settings.smtpTo')}</label>
                            <input type="text" id="alert-email-to" value="${this.escHtml(emailTo)}" placeholder="admin@example.com">
                        </div>
                    </div>
                    <button class="btn-primary" onclick="settingsPage.saveAlertNotify()" style="margin-top:12px;">${I18n.t('settings.saveAlertConfig')}</button>
                    <button class="btn-secondary" onclick="settingsPage.sendTestEmail()" style="margin-top:12px;margin-left:8px;">${I18n.t('settings.sendTestEmail')}</button>
                </div>
            </div>`;
        }

        return html;
    }

    // 渲染暂停知识库卡片
    renderKbPauseCard() {
        const kb = this.kb || {};
        const paused = !!kb.paused;
        const permanent = !!kb.permanent;
        const remainingDays = kb.remaining_days;

        let badgeText, badgeColor, body;
        if (!paused) {
            badgeText = I18n.t('settings.kbPauseRunning');
            badgeColor = 'var(--success)';
            body = `
                <p style="color:var(--text-muted);font-size:13px;margin:0 0 12px 0;">${I18n.t('settings.kbPauseHint')}</p>
                <div style="display:flex;gap:12px;align-items:center;flex-wrap:wrap;">
                    <label style="font-size:13px;color:var(--text-secondary);">${I18n.t('settings.kbPauseDays')}</label>
                    <input id="kb-pause-days" type="number" min="1" max="3650" value="7"
                           style="width:90px;padding:6px 10px;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);color:var(--text-primary);font-size:14px;">
                    <button class="btn-primary btn-sm" onclick="settingsPage.pauseKb(false)">${I18n.t('settings.kbPauseDaysBtn')}</button>
                    <button class="btn-secondary btn-sm" onclick="settingsPage.pauseKb(true)" style="margin-left:auto;">${I18n.t('settings.kbPausePermanent')}</button>
                </div>`;
        } else {
            const remainText = permanent
                ? I18n.t('settings.kbPausePermanentLabel')
                : I18n.t('settings.kbPauseRemaining', { days: remainingDays });
            badgeText = I18n.t('settings.kbPausePaused');
            badgeColor = 'var(--warning, #f59e0b)';
            body = `
                <p style="color:var(--text-secondary);font-size:13px;margin:0 0 12px 0;">${remainText}</p>
                <button class="btn-primary btn-sm" onclick="settingsPage.resumeKb()">${I18n.t('settings.kbPauseResume')}</button>`;
        }

        return `
        <div class="card" style="border-left:3px solid ${badgeColor};">
            <div class="card-header">
                <span class="card-title">${I18n.t('settings.kbPauseTitle')}</span>
                <span style="margin-left:auto;font-size:12px;color:${badgeColor};">${badgeText}</span>
            </div>
            <div style="padding:16px;">
                ${body}
            </div>
        </div>`;
    }

    async pauseKb(permanent) {
        let payload;
        if (permanent) {
            if (!confirm(I18n.t('settings.kbPausePermanentConfirm'))) return;
            payload = { permanent: true };
        } else {
            const inp = document.getElementById('kb-pause-days');
            const days = parseInt(inp ? inp.value : 0, 10);
            if (!days || days <= 0) {
                showToast(I18n.t('settings.kbPauseInvalidDays'), 'error');
                return;
            }
            payload = { days };
        }
        try {
            const res = await fetch('/api/knowledge/pause', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload),
            });
            if (!res.ok) throw new Error('HTTP ' + res.status);
            showToast(I18n.t('settings.kbPauseDone'));
            await this.loadKbStatus();
            this.render();
        } catch (e) {
            showToast(I18n.t('settings.kbPauseFailed') + (e.message || ''), 'error');
        }
    }

    async resumeKb() {
        try {
            const res = await fetch('/api/knowledge/resume', { method: 'POST' });
            if (!res.ok) throw new Error('HTTP ' + res.status);
            showToast(I18n.t('settings.kbResumeDone'));
            await this.loadKbStatus();
            this.render();
        } catch (e) {
            showToast(I18n.t('settings.kbResumeFailed') + (e.message || ''), 'error');
        }
    }

    // 渲染 wr-proxy 优化特性开关卡片
    renderFeatureToggles() {
        const featDefs = [
            {
                key: 'feature_dynamic_content_last',
                title: I18n.t("settings.dynamicContentLast"),
                shortDesc: I18n.t("settings.dynamicContentLastDesc"),
                detail: I18n.t("settings.dynamicContentLastDetail"),
                icon: '🔀',
            },
            {
                key: 'feature_token_compression',
                title: I18n.t("settings.tokenCompression"),
                shortDesc: I18n.t("settings.tokenCompressionDesc"),
                detail: I18n.t("settings.tokenCompressionDetail"),
                icon: '📦',
            },
            {
                key: 'feature_session_compression',
                title: I18n.t("settings.sessionCompression"),
                shortDesc: I18n.t("settings.sessionCompressionDesc"),
                detail: I18n.t("settings.sessionCompressionDetail"),
                icon: '📉',
            },
        ];

        const dbSettings = {};
        this.settings.forEach(s => { dbSettings[s.key] = s; });

        return `
        <div class="card" style="border-left:3px solid var(--accent);">
            <div class="card-header">
                <span class="card-title">${I18n.t('settings.featureToggles')}</span>
                <button class="btn-primary btn-sm" onclick="settingsPage.reloadProxy()" id="btn-reload-proxy">${I18n.t('settings.reloadProxy')}</button>
            </div>
            <div style="padding:16px;">
                <p style="color:var(--text-muted);font-size:12px;margin-bottom:16px;">${I18n.t('settings.featureHint')}</p>
                ${featDefs.map(feat => {
                    const s = dbSettings[feat.key];
                    const enabled = s ? s.value : false;
                    const desc = s ? s.description : feat.shortDesc;
                    return `
                <div style="margin-bottom:16px;padding:12px;background:rgba(255,255,255,0.02);border-radius:8px;border:1px solid var(--border);">
                    <div style="display:flex;align-items:center;gap:12px;margin-bottom:8px;">
                        <label class="toggle-switch">
                            <input type="checkbox" ${enabled ? 'checked' : ''}
                                   onchange="settingsPage.toggleFeatureState('${feat.key}', this.checked, this)">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-weight:600;font-size:14px;${enabled ? 'color:var(--success)' : ''}">${feat.title}</span>
                        <span class="state-label" style="margin-left:auto;font-size:12px;color:${enabled ? 'var(--success)' : 'var(--text-muted)'};">${enabled ? I18n.t("settings.enabled") : I18n.t("settings.disabled")}</span>
                    </div>
                    <p style="color:var(--text-muted);font-size:13px;margin:0 0 6px 0;">${this.escHtml(feat.shortDesc)}</p>
                    <details style="color:var(--text-secondary);font-size:12px;margin:0;">
                        <summary style="cursor:pointer;color:var(--accent);">${I18n.t('settings.detailToggle')}</summary>
                        <p style="margin:6px 0 0 0;line-height:1.6;">${this.escHtml(feat.detail)}</p>
                        ${desc !== feat.shortDesc ? `<p style="margin:6px 0 0 0;color:var(--text-muted);font-size:11px;">${I18n.t('settings.dbDescription')}${this.escHtml(desc)}</p>` : ''}
                    </details>
                </div>`;
                }).join('')}
            </div>
        </div>`;
    }

    async seedFeatures() {
        try {
            const result = await API.post('/settings/seed-features', {});
            showToast(result.message);
            this.loadSettings();
        } catch (e) {
            showToast(I18n.t("settings.initFailed") + (e.message || ''), 'error');
        }
    }

    // 渲染知识提取配置卡片（extract_provider_id / extract_model / extract_timeout_sec / extract_batch_size / extract_force_json_object）
    renderExtractConfig() {
        const get = (k, def) => {
            const s = this.settings.find(x => x.key === k);
            return s ? s.value : def;
        };
        const providerId  = parseInt(get('extract_provider_id', 0)) || 0;
        const model       = get('extract_model', '');
        const timeoutSec  = parseInt(get('extract_timeout_sec', 120)) || 120;
        const batchSize   = parseInt(get('extract_batch_size', 5)) || 5;
        const forceJson   = !!get('extract_force_json_object', false);

        // provider 选项 + 当前选中 provider 的可用模型
        const selected = this.providers.find(p => p.id === providerId);
        const providerOpts = ['<option value="0">— 自动（按路由顺序）—</option>']
            .concat(this.providers.map(p => {
                const dot = (p.status === 'healthy') ? '🟢' : (p.status === 'unhealthy' ? '🟡' : '🔴');
                return `<option value="${p.id}" ${p.id === providerId ? 'selected' : ''}>${dot} #${p.id} ${this.escHtml(p.name)} (${p.status})</option>`;
            })).join('');

        const modelOpts = (() => {
            if (!selected || !selected.models || !Array.isArray(selected.models) || selected.models.length === 0) {
                return '';
            }
            return selected.models.map(m => `<option value="${this.escHtml(m)}" ${m === model ? 'selected' : ''}>${this.escHtml(m)}</option>`).join('');
        })();

        return `
        <div class="card" style="border-left:3px solid var(--info, #4a90e2);">
            <div class="card-header">
                <span class="card-title">🧠 知识提取（LLM 评估）配置</span>
                <button class="btn-secondary btn-sm" onclick="settingsPage.runExtractNow()">立即跑一次</button>
            </div>
            <div style="padding:16px;">
                <p style="color:var(--text-muted);font-size:12px;margin-bottom:16px;">
                    用于把 wr_knowledge_raw 里的对话条目提炼为结构化知识。修改后下一轮提取（5min 周期或手动触发）即生效，无需重启 wr-proxy。
                </p>
                <div style="display:grid;grid-template-columns:160px 1fr;gap:12px 16px;align-items:center;">
                    <label>评估 Provider</label>
                    <select id="extract-provider-id" onchange="settingsPage.onExtractProviderChange()" style="padding:6px 10px;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);color:var(--text-primary);">
                        ${providerOpts}
                    </select>

                    <label>评估模型</label>
                    <div style="display:flex;gap:8px;">
                        ${modelOpts ? `<select id="extract-model-select" onchange="document.getElementById('extract-model').value=this.value" style="padding:6px 10px;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);color:var(--text-primary);min-width:180px;"><option value="">— 选择 —</option>${modelOpts}</select>` : ''}
                        <input type="text" id="extract-model" value="${this.escHtml(model)}" placeholder="如 ark-code-latest / qwen3-coder-flash" style="flex:1;padding:6px 10px;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);color:var(--text-primary);">
                    </div>

                    <label>单次超时 (秒)</label>
                    <input type="number" id="extract-timeout-sec" value="${timeoutSec}" min="10" max="600" style="width:120px;padding:6px 10px;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);color:var(--text-primary);">

                    <label>每轮批量大小</label>
                    <input type="number" id="extract-batch-size" value="${batchSize}" min="1" max="20" style="width:120px;padding:6px 10px;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);color:var(--text-primary);">

                    <label>强制 JSON Object</label>
                    <label style="display:flex;align-items:center;gap:8px;cursor:pointer;">
                        <input type="checkbox" id="extract-force-json" ${forceJson ? 'checked' : ''}>
                        <span style="color:var(--text-muted);font-size:12px;">勾选后请求带 response_format=json_object（仅 OpenAI/DeepSeek/Qwen 兼容；ARK/Doubao 会 400）</span>
                    </label>
                </div>
                <div style="margin-top:16px;display:flex;gap:8px;">
                    <button class="btn-primary" onclick="settingsPage.saveExtractConfig()">保存</button>
                    <span id="extract-save-hint" style="color:var(--text-muted);font-size:12px;align-self:center;"></span>
                </div>
            </div>
        </div>`;
    }

    onExtractProviderChange() {
        // 切换 provider 后重渲染该卡片，让"评估模型"下拉刷新成新 provider 支持的模型列表
        const newId = parseInt(document.getElementById('extract-provider-id').value) || 0;
        const cur = this.settings.find(s => s.key === 'extract_provider_id');
        if (cur) cur.value = newId;
        else this.settings.push({key: 'extract_provider_id', value: newId, value_type: 'int'});
        // 同时把当前模型 input 留空，提示用户重新选
        const inp = document.getElementById('extract-model');
        if (inp) inp.value = '';
        const cm = this.settings.find(s => s.key === 'extract_model');
        if (cm) cm.value = '';
        this.render();
    }

    async saveExtractConfig() {
        const providerId = parseInt(document.getElementById('extract-provider-id').value) || 0;
        const model      = document.getElementById('extract-model').value.trim();
        const timeoutSec = parseInt(document.getElementById('extract-timeout-sec').value) || 120;
        const batchSize  = parseInt(document.getElementById('extract-batch-size').value) || 5;
        const forceJson  = document.getElementById('extract-force-json').checked;

        if (providerId > 0 && !model) {
            showToast('指定了 Provider，请同时填写评估模型', 'error');
            return;
        }

        const hint = document.getElementById('extract-save-hint');
        if (hint) hint.textContent = '保存中...';

        await this.saveSetting('extract_provider_id', providerId);
        await this.saveSetting('extract_model', model);
        await this.saveSetting('extract_timeout_sec', timeoutSec);
        await this.saveSetting('extract_batch_size', batchSize);
        await this.saveSetting('extract_force_json_object', forceJson);

        if (hint) hint.textContent = '已保存，下一轮提取生效';
        await this.loadSettings();
    }

    async runExtractNow() {
        try {
            const proxyBase = (this.settings.find(s => s.key === 'gateway_url')?.value) || 'http://localhost:5051';
            const resp = await fetch(`${proxyBase}/admin/knowledge_extract`, {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({batch_size: parseInt(document.getElementById('extract-batch-size')?.value) || 5}),
            });
            const result = await resp.json();
            if (result.error) {
                showToast('提取失败: ' + result.error, 'error');
            } else {
                showToast(result.message || `已处理 ${result.processed} 条`);
            }
        } catch (e) {
            showToast('调用失败: ' + e.message, 'error');
        }
    }

    // 渲染六维度复杂度配置编辑器
    renderComplexityConfig() {
        const s = this.settings.find(x => x.key === 'smart_complexity_config');
        if (!s) return '';

        const cfg = s.value || {};
        const tier = cfg.tier_thresholds || { simple_max: 0.20, moderate_max: 0.45 };

        // 分级阈值栏
        const tierHtml = `
        <div style="margin-bottom:16px;padding:12px 16px;background:var(--bg-primary);border-radius:8px;border:1px solid var(--border);display:flex;align-items:center;gap:16px;flex-wrap:wrap;">
            <span style="font-weight:600;color:var(--accent);">${I18n.t('settings.tierThresholds')}</span>
            <span style="color:var(--text-muted);font-size:13px;">
                <input type="number" step="0.01" min="0" max="1" value="${tier.simple_max}"
                    onchange="settingsPage.updateComplexityTier('simple_max', this.value)"
                    style="width:64px;padding:4px 8px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:13px;text-align:center;">
                <span style="margin:0 4px;">${I18n.t('settings.simpleToEconomy')}</span>
                &nbsp;│&nbsp;
                <input type="number" step="0.01" min="0" max="1" value="${tier.moderate_max}"
                    onchange="settingsPage.updateComplexityTier('moderate_max', this.value)"
                    style="width:64px;padding:4px 8px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:13px;text-align:center;">
                <span style="margin:0 4px;">${I18n.t('settings.moderateToStandard')}</span>
                &nbsp;≥&nbsp;<span style="font-weight:500;">${tier.moderate_max}</span>&nbsp;→ premium
            </span>
        </div>`;

        // 六维度定义
        const dims = [
            {
                key: 'input_length', icon: '📏', title: I18n.t('settings.inputLength').replace('📏 ', ''),
                desc: I18n.t("settings.inputLengthDesc"),
                type: 'levels', unit: I18n.t("settings.chars"), field: 'max_chars',
            },
            {
                key: 'multi_turn', icon: '💬', title: I18n.t('settings.multiTurn').replace('💬 ', ''),
                desc: I18n.t("settings.multiTurnDesc"),
                type: 'levels', unit: I18n.t("settings.turns"), field: 'max_msgs',
            },
            {
                key: 'code_detection', icon: '💻', title: I18n.t('settings.codeDetection').replace('💻 ', ''),
                desc: I18n.t("settings.codeDetectionDesc"),
                type: 'keywords', keywordsField: 'keywords', scoreField: 'score',
            },
            {
                key: 'tools_detection', icon: '🔧', title: I18n.t('settings.toolsDetection').replace('🔧 ', ''),
                desc: I18n.t("settings.toolsDetectionDesc"),
                type: 'scores', toolsScoreField: 'tools_score', functionsScoreField: 'functions_score',
            },
            {
                key: 'reasoning_keywords', icon: '🧠', title: I18n.t('settings.reasoningKeywords').replace('🧠 ', ''),
                desc: I18n.t("settings.reasoningKeywordsDesc"),
                type: 'keywords', keywordsField: 'keywords', scoreField: 'score',
            },
            {
                key: 'system_prompt', icon: '📋', title: I18n.t('settings.systemPrompt').replace('📋 ', ''),
                desc: I18n.t("settings.systemPromptDesc"),
                type: 'threshold', thresholdField: 'threshold_chars', scoreField: 'score',
            },
        ];

        const inputLen = cfg.input_length || { enabled: true, levels: [
            { max_chars: 200, score: 0.05 }, { max_chars: 800, score: 0.12 },
            { max_chars: 2000, score: 0.20 }, { max_chars: 0, score: 0.30 },
        ]};
        const multiTurn = cfg.multi_turn || { enabled: true, levels: [
            { max_msgs: 2, score: 0.0 }, { max_msgs: 5, score: 0.08 },
            { max_msgs: 10, score: 0.15 }, { max_msgs: 0, score: 0.20 },
        ]};
        const codeDet = cfg.code_detection || { enabled: true, score: 0.15, keywords: ['```', 'def ', 'function ', 'class ', 'import ', 'return '] };
        const toolsDet = cfg.tools_detection || { enabled: true, tools_score: 0.20, functions_score: 0.15 };
        const reasonDet = cfg.reasoning_keywords || { enabled: true, score: 0.12, keywords: ['explain', 'analyze', 'reason', 'prove', 'calculate', 'derive', 'compare', 'evaluate', 'critique', 'why', 'cause', 'principle', 'logic', 'steps', 'plan', 'strategy', 'design'] };
        const sysPrompt = cfg.system_prompt || { enabled: true, threshold_chars: 500, score: 0.08 };
        const dimConfigs = {
            input_length: inputLen, multi_turn: multiTurn,
            code_detection: codeDet, tools_detection: toolsDet,
            reasoning_keywords: reasonDet, system_prompt: sysPrompt,
        };

        // 渲染单个维度卡片
        const renderDim = (dim) => {
            const dc = dimConfigs[dim.key];
            const on = dc.enabled !== false;
            const accentColor = on ? 'var(--success)' : 'var(--text-muted)';

            let contentHtml = '';
            if (dim.type === 'levels') {
                const levels = dc.levels || [];
                contentHtml = `<table style="width:100%;font-size:13px;">
                    <thead><tr style="color:var(--text-muted);border-bottom:1px solid var(--border);">
                        <th style="padding:4px 8px;text-align:left;font-weight:500;">${I18n.t('settings.condition')}</th>
                        <th style="padding:4px 8px;text-align:center;font-weight:500;">${I18n.t('settings.score')}</th>
                    </tr></thead>
                    <tbody>${levels.map((lv, i) => {
                        const prev = i > 0 ? levels[i-1][dim.field] : 0;
                        const curr = lv[dim.field];
                        let label;
                        if (curr === 0 && i === levels.length - 1) {
                            label = `${dim.unit} ≥ ${prev}`;
                        } else if (i === 0) {
                            label = `${dim.unit} &lt; ${curr}`;
                        } else {
                            label = `${prev} ~ ${curr}`;
                        }
                        return `<tr style="border-bottom:1px solid var(--border);">
                            <td style="padding:6px 8px;color:var(--text-secondary);">${label}</td>
                            <td style="padding:6px 8px;text-align:center;">
                                <input type="number" step="0.01" min="0" max="1" value="${lv.score}"
                                    onchange="settingsPage.updateComplexityLevel('${dim.key}', ${i}, 'score', this.value)"
                                    style="width:60px;padding:4px 6px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:13px;text-align:center;">
                            </td>
                        </tr>`;
                    }).join('')}</tbody>
                </table>`;
            } else if (dim.type === 'keywords') {
                const kw = (dc.keywords || []).join(', ');
                contentHtml = `<div style="display:flex;gap:12px;align-items:flex-start;flex-wrap:wrap;">
                    <div style="flex:1;min-width:200px;">
                        <div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;">${I18n.t('settings.keywordsCommaSeparated')}</div>
                        <input type="text" value="${this.escHtml(kw)}"
                            onchange="settingsPage.updateComplexityKeywords('${dim.key}', this.value)"
                            style="width:100%;padding:5px 8px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:12px;font-family:monospace;">
                    </div>
                    <div style="min-width:80px;">
                        <div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;">${I18n.t('settings.hitScore')}</div>
                        <input type="number" step="0.01" min="0" max="1" value="${dc[dim.scoreField]}"
                            onchange="settingsPage.updateComplexityField('${dim.key}', '${dim.scoreField}', this.value)"
                            style="width:64px;padding:5px 6px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:13px;text-align:center;">
                    </div>
                </div>`;
            } else if (dim.type === 'scores') {
                contentHtml = `<div style="display:flex;gap:20px;">
                    <div>
                        <div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;">${I18n.t('settings.toolsScore')}</div>
                        <input type="number" step="0.01" min="0" max="1" value="${dc.tools_score}"
                            onchange="settingsPage.updateComplexityField('tools_detection', 'tools_score', this.value)"
                            style="width:64px;padding:5px 6px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:13px;text-align:center;">
                    </div>
                    <div>
                        <div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;">${I18n.t('settings.functionsScore')}</div>
                        <input type="number" step="0.01" min="0" max="1" value="${dc.functions_score}"
                            onchange="settingsPage.updateComplexityField('tools_detection', 'functions_score', this.value)"
                            style="width:64px;padding:5px 6px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:13px;text-align:center;">
                    </div>
                </div>`;
            } else if (dim.type === 'threshold') {
                contentHtml = `<div style="display:flex;gap:20px;">
                    <div>
                        <div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;">${I18n.t('settings.charThreshold')}</div>
                        <input type="number" min="0" value="${dc.threshold_chars}"
                            onchange="settingsPage.updateComplexityField('system_prompt', 'threshold_chars', this.value)"
                            style="width:80px;padding:5px 6px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:13px;text-align:center;">
                    </div>
                    <div>
                        <div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;">${I18n.t('settings.hitScore')}</div>
                        <input type="number" step="0.01" min="0" max="1" value="${dc.score}"
                            onchange="settingsPage.updateComplexityField('system_prompt', 'score', this.value)"
                            style="width:64px;padding:5px 6px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:13px;text-align:center;">
                    </div>
                </div>`;
            }

            return `
                <div style="padding:12px 14px;background:var(--bg-primary);border-radius:8px;border-left:3px solid ${accentColor};border-top:1px solid var(--border);border-right:1px solid var(--border);border-bottom:1px solid var(--border);">
                    <div style="display:flex;align-items:center;gap:10px;margin-bottom:6px;">
                        <label class="toggle-switch">
                            <input type="checkbox" ${on ? 'checked' : ''}
                                   onchange="settingsPage.toggleComplexityDim('${dim.key}', this.checked)">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-weight:600;font-size:14px;">${dim.icon} ${dim.title}</span>
                        <span class="state-label" style="font-size:12px;color:${accentColor};margin-left:auto;">${on ? I18n.t("settings.enabledState") : I18n.t("settings.disabledState")}</span>
                    </div>
                    ${on ? contentHtml : ''}
                    <details style="margin-top:6px;color:var(--text-muted);font-size:12px;">
                        <summary style="cursor:pointer;color:var(--accent);">${I18n.t('settings.descToggle')}</summary>
                        <p style="margin:4px 0 0 0;line-height:1.6;">${dim.desc}</p>
                    </details>
                </div>`;
        };

        // 2×3 网格布局
        let dimsHtml = '<div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-top:12px;">';
        for (const dim of dims) {
            dimsHtml += renderDim(dim);
        }
        dimsHtml += '</div>';

        return `
        <div class="card" style="border-left:3px solid #8b5cf6;">
            <div class="card-header">
                <span class="card-title">${I18n.t('settings.complexityConfig')}</span>
                <div>
                    <button class="btn-secondary btn-sm" onclick="settingsPage.resetComplexityConfig()" style="margin-right:8px;">${I18n.t('settings.restoreDefault')}</button>
                    <button class="btn-primary btn-sm" onclick="settingsPage.saveComplexityConfig()" style="margin-right:8px;">${I18n.t('settings.saveConfig')}</button>
                    <button class="btn-secondary btn-sm" onclick="settingsPage.reloadProxy()">${I18n.t('settings.reload')}</button>
                </div>
            </div>
            <div style="padding:16px;">
                <p style="color:var(--text-muted);font-size:12px;margin-bottom:12px;">
                    ${I18n.t('settings.complexityHint')}
                </p>
                ${tierHtml}
                ${dimsHtml}
            </div>
        </div>`;
    }

    async saveComplexityConfig() {
        const s = this.settings.find(x => x.key === 'smart_complexity_config');
        if (!s) return;
        await this.saveSetting('smart_complexity_config', s.value);
        await this.loadSettings();
        showToast(I18n.t('settings.complexitySaved'));
    }

    async resetComplexityConfig() {
        if (!confirm(I18n.t('settings.confirmResetComplexity'))) return;
        const defaults = {
            "tier_thresholds": { "simple_max": 0.20, "moderate_max": 0.45 },
            "input_length": { "enabled": true, "levels": [
                {"max_chars": 200, "score": 0.05}, {"max_chars": 800, "score": 0.12},
                {"max_chars": 2000, "score": 0.20}, {"max_chars": 0, "score": 0.30}
            ]},
            "multi_turn": { "enabled": true, "levels": [
                {"max_msgs": 2, "score": 0.0}, {"max_msgs": 5, "score": 0.08},
                {"max_msgs": 10, "score": 0.15}, {"max_msgs": 0, "score": 0.20}
            ]},
            "code_detection": { "enabled": true, "score": 0.15, "keywords": ["```", "def ", "function ", "class ", "import ", "return "] },
            "tools_detection": { "enabled": true, "tools_score": 0.20, "functions_score": 0.15 },
            "reasoning_keywords": { "enabled": true, "score": 0.12, "keywords": ["explain", "analyze", "reason", "prove", "calculate", "derive", "compare", "evaluate", "critique", "why", "cause", "principle", "logic", "steps", "plan", "strategy", "design"] },
            "system_prompt": { "enabled": true, "threshold_chars": 500, "score": 0.08 },
        };
        await this.saveSetting('smart_complexity_config', defaults);
        await this.loadSettings();
        showToast(I18n.t("settings.complexityReset"));
    }

    updateComplexityTier(field, value) {
        const s = this.settings.find(x => x.key === 'smart_complexity_config');
        if (!s) return;
        s.value.tier_thresholds[field] = parseFloat(value) || 0;
        this.render();
    }

    updateComplexityLevel(dimKey, index, field, value) {
        const s = this.settings.find(x => x.key === 'smart_complexity_config');
        if (!s) return;
        const dim = s.value[dimKey];
        if (dim && dim.levels && dim.levels[index]) {
            dim.levels[index][field] = parseFloat(value) || 0;
        }
        this.render();
    }

    updateComplexityKeywords(dimKey, rawValue) {
        const s = this.settings.find(x => x.key === 'smart_complexity_config');
        if (!s) return;
        const dim = s.value[dimKey];
        if (dim) {
            dim.keywords = rawValue.split(',').map(k => k.trim()).filter(k => k.length > 0);
        }
        this.render();
    }

    updateComplexityField(dimKey, field, value) {
        const s = this.settings.find(x => x.key === 'smart_complexity_config');
        if (!s) return;
        const dim = s.value[dimKey];
        if (dim) {
            dim[field] = parseFloat(value) || 0;
        }
        this.render();
    }

    toggleComplexityDim(dimKey, enabled) {
        const s = this.settings.find(x => x.key === 'smart_complexity_config');
        if (!s) return;
        const dim = s.value[dimKey];
        if (dim) {
            dim.enabled = enabled;
        }
        this.render();
    }

    async toggleFeature(key, value) {
        await this.saveSetting(key, value);
    }

    async toggleFeatureState(key, value, el) {
        const label = el.closest('label').parentElement.querySelector('.state-label');
        if (label) {
            label.textContent = value ? I18n.t("settings.enabled") : I18n.t("settings.disabled");
            label.style.color = value ? 'var(--success)' : 'var(--text-muted)';
        }
        await this.toggleFeature(key, value);
    }

    async reloadProxy() {
        const btn = document.getElementById('btn-reload-proxy');
        if (btn) { btn.disabled = true; btn.textContent = I18n.t("settings.reloading"); }
        try {
            const result = await API.post('/settings/reload-proxy', {});
            if (result.success) {
                showToast('🔄 ' + result.message + (result.detail ? ' — ' + result.detail : ''));
            } else {
                showToast(I18n.t('settings.reloadFailed') + result.message, 'error');
            }
        } catch (e) {
            showToast(I18n.t("settings.reloadFailed") + (e.message || I18n.t("common.networkError")), 'error');
        } finally {
            if (btn) { btn.disabled = false; btn.textContent = I18n.t('settings.reloadProxy'); }
        }
    }

    async saveLogRetention() {
        const input = document.getElementById('log-retention-input');
        const value = parseInt(input.value);
        if (isNaN(value) || value < 1) {
            showToast(I18n.t("settings.retentionDaysError"), 'error');
            return;
        }
        await this.saveSetting('log_retention_days', value);
    }

    addHealthConfig() {
        const s = this.settings.find(x => x.key === 'health_test_configs');
        const configs = Array.isArray(s.value) ? s.value : [];
        configs.push({ domain: '', name: '', endpoint: '/v1/chat/completions', body: '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}],"max_tokens":1}' });
        s.value = configs;
        this.render();
    }

    removeHealthConfig(index) {
        const s = this.settings.find(x => x.key === 'health_test_configs');
        const configs = Array.isArray(s.value) ? s.value : [];
        configs.splice(index, 1);
        s.value = configs;
        this.render();
    }

    updateHealthConfig(index, field, value) {
        const s = this.settings.find(x => x.key === 'health_test_configs');
        const configs = Array.isArray(s.value) ? s.value : [];
        if (configs[index]) {
            configs[index][field] = value;
            s.value = configs;
        }
    }

    async saveHealthConfigs() {
        const s = this.settings.find(x => x.key === 'health_test_configs');
        if (!s) return;
        await this.saveSetting('health_test_configs', s.value);
    }

    async resetHealthConfigs() {
        if (!confirm(I18n.t("settings.confirmResetHealthConfig"))) return;
        const defaults = [
            { domain: 'api.openai.com', name: 'OpenAI', endpoint: '/v1/chat/completions', body: '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}],"max_tokens":1}' },
            { domain: 'api.anthropic.com', name: 'Anthropic', endpoint: '/v1/messages', body: '{"model":"claude-3-haiku-20240307","messages":[{"role":"user","content":"hi"}],"max_tokens":1}' },
            { domain: 'api.deepseek.com', name: 'DeepSeek', endpoint: '/v1/chat/completions', body: '{"model":"deepseek-chat","messages":[{"role":"user","content":"hi"}],"max_tokens":1}' },
            { domain: 'dashscope.aliyuncs.com', name: I18n.t("vendor.qwen"), endpoint: '/compatible-mode/v1/chat/completions', body: '{"model":"qwen-turbo","messages":[{"role":"user","content":"hi"}],"max_tokens":1}' },
            { domain: 'open.bigmodel.cn', name: I18n.t("vendor.zhipu"), endpoint: '/v4/chat/completions', body: '{"model":"glm-4-flash","messages":[{"role":"user","content":"hi"}],"max_tokens":1}' },
        ];
        await this.saveSetting('health_test_configs', defaults);
    }

    async saveAlertNotify() {
        const wechatSendkey = document.getElementById('alert-wechat-sendkey').value.trim();
        const smtpHost = document.getElementById('alert-smtp-host').value.trim();
        const smtpPort = parseInt(document.getElementById('alert-smtp-port').value) || 587;
        const smtpUser = document.getElementById('alert-smtp-user').value.trim();
        const smtpPass = document.getElementById('alert-smtp-password').value;
        const smtpFrom = document.getElementById('alert-smtp-from').value.trim();
        const emailTo = document.getElementById('alert-email-to').value.trim();

        if (!wechatSendkey && !smtpHost) {
            showToast(I18n.t("settings.alertConfigRequired"), 'error');
            return;
        }
        if (smtpHost && !emailTo) {
            showToast(I18n.t("settings.smtpToRequired"), 'error');
            return;
        }

        const updates = [
            ['alert_wechat_sendkey', wechatSendkey],
            ['alert_smtp_host', smtpHost],
            ['alert_smtp_port', smtpPort],
            ['alert_smtp_user', smtpUser],
            ['alert_smtp_password', smtpPass],
            ['alert_smtp_from', smtpFrom],
            ['alert_email_to', emailTo],
        ];

        for (const [key, value] of updates) {
            await this.saveSetting(key, value);
        }
        showToast(I18n.t("settings.alertConfigSaved"));
    }

    async sendTestEmail() {
        // 先保存当前表单值
        const smtpHost = document.getElementById('alert-smtp-host').value.trim();
        const emailTo = document.getElementById('alert-email-to').value.trim();
        const smtpUser = document.getElementById('alert-smtp-user').value.trim();
        const smtpPass = document.getElementById('alert-smtp-password').value;

        if (!smtpHost || !emailTo || !smtpUser || !smtpPass) {
            showToast(I18n.t("settings.smtpConfigRequired"), 'error');
            return;
        }

        // 先保存配置再发送测试邮件
        await this.saveAlertNotify();

        try {
            showToast(I18n.t("settings.sendingTestEmail"));
            const result = await API.post('/settings/test-email', {});
            if (result.success) {
                showToast('📧 ' + result.message);
            } else {
                showToast(I18n.t("settings.sendFailed") + result.message, 'error');
            }
        } catch (e) {
            showToast(I18n.t("settings.sendFailed") + (e.message || I18n.t("common.networkError")), 'error');
        }
    }

    renderSettingsTable() {
        let filtered = this.settings;
        if (this.filterCategory) {
            filtered = filtered.filter(s => s.category === this.filterCategory);
        }
        // 已有专用卡片的设置从表格中隐藏
        filtered = filtered.filter(s =>
            s.key !== 'log_retention_days' &&
            s.key !== 'health_test_configs' &&
            s.key !== 'proxy_enabled' &&
            s.key !== 'proxy_url' &&
            s.key !== 'gateway_url' &&
            s.key !== 'routing_strategy' &&
            s.key !== 'default_timeout' &&
            s.key !== 'max_failover' &&
            s.key !== 'max_retry_count' &&
            s.key !== 'alert_wechat_sendkey' &&
            s.key !== 'alert_smtp_host' &&
            s.key !== 'alert_smtp_port' &&
            s.key !== 'alert_smtp_user' &&
            s.key !== 'alert_smtp_password' &&
            s.key !== 'alert_smtp_from' &&
            s.key !== 'alert_email_to' &&
            s.key !== 'feature_dynamic_content_last' &&
            s.key !== 'feature_token_compression' &&
            s.key !== 'feature_session_compression' &&
            s.key !== 'smart_complexity_config' &&
            !s.key.startsWith('extract_')
        );

        if (filtered.length === 0) {
            return `<div class="empty-state"><p>${I18n.t('settings.noSettings')}</p></div>`;
        }

        const categoryLabels = {
            'general': I18n.t("settings.generalCat"),
            'proxy': I18n.t("settings.proxyCat"),
            'monitor': I18n.t("settings.monitorCat"),
            'alert': I18n.t("settings.alertCat"),
            'advanced': I18n.t("settings.advancedCat"),
        };

        let html = '<div class="table-wrap"><table><thead><tr>'
            + `<th>${I18n.t('settings.keyName')}</th><th>${I18n.t('settings.value')}</th><th>${I18n.t('settings.typeCol')}</th><th>${I18n.t('settings.categoryCol')}</th><th>${I18n.t('settings.descCol')}</th><th>${I18n.t('common.actions')}</th>`
            + '</tr></thead><tbody>';

        for (const s of filtered) {
            const catLabel = categoryLabels[s.category] || s.category;
            html += `
                <tr>
                    <td><code class="setting-key">${this.escHtml(s.key)}</code></td>
                    <td>${this.renderSettingControl(s, false)}</td>
                    <td><span class="badge badge-info">${this.escHtml(this.VALUE_TYPE_LABELS[s.value_type] || s.value_type)}</span></td>
                    <td>${catLabel}</td>
                    <td class="setting-notes">${this.escHtml(s.description || '-')}</td>
                    <td>
                        ${s.editable
                            ? `<button class="btn-sm" onclick="settingsPage.editSetting('${this.escHtml(s.key)}')">${I18n.t('common.edit')}</button>`
                            : '<span style="color:var(--text-muted)">-</span>'}
                    </td>
                </tr>`;
        }

        html += '</tbody></table></div>';
        return html;
    }

    renderSettingControl(setting, compact) {
        const val = setting.value;
        const key = setting.key;

        switch (setting.value_type) {
            case 'bool':
                return `
                    <label class="toggle-switch ${compact ? 'compact' : ''}">
                        <input type="checkbox" ${val ? 'checked' : ''}
                               onchange="this.closest('label').querySelector('.toggle-label').textContent=this.checked?I18n.t('settings.toggleEnabled'):I18n.t('settings.toggleDisabled');settingsPage.toggleBool('${key}',this.checked)"
                               ${setting.editable ? '' : 'disabled'}>
                        <span class="toggle-slider"></span>
                        <span class="toggle-label">${val ? I18n.t("common.on") : I18n.t("common.off")}</span>
                    </label>`;
            case 'int':
            case 'float':
                return `
                    <input type="number"
                           class="setting-input ${compact ? 'compact' : ''}"
                           value="${val}"
                           step="${setting.value_type === 'float' ? '0.01' : '1'}"
                           data-key="${key}"
                           ${setting.editable ? '' : 'disabled'}
                           onchange="settingsPage.updateNumber('${key}', this.value, '${setting.value_type}')">`;
            case 'json':
                const display = typeof val === 'object' ? JSON.stringify(val, null, 2) : val;
                return `
                    <textarea class="setting-textarea ${compact ? 'compact' : ''}"
                              data-key="${key}"
                              rows="${compact ? 2 : 4}"
                              ${setting.editable ? '' : 'disabled'}
                              onchange="settingsPage.updateJson('${key}', this.value)">${this.escHtml(display)}</textarea>`;
            default:
                if (key === 'routing_strategy') {
                    const strategies = ['smart', 'priority', 'round_robin', 'least_latency', 'cost_first'];
                    return `
                        <select class="setting-input" data-key="${key}"
                                onchange="settingsPage.updateString('${key}', this.value)"
                                ${setting.editable ? '' : 'disabled'}>
                            ${strategies.map(s =>
                                `<option value="${s}" ${val === s ? 'selected' : ''}>${this.ROUTING_STRATEGY_LABELS[s]}</option>`
                            ).join('')}
                        </select>`;
                }
                return `
                    <input type="text"
                           class="setting-input ${compact ? 'compact' : ''}"
                           value="${this.escHtml(String(val))}"
                           data-key="${key}"
                           ${setting.editable ? '' : 'disabled'}
                           onchange="settingsPage.updateString('${key}', this.value)">`;
        }
    }

    onCategoryFilter() {
        this.filterCategory = document.getElementById('settings-category-filter').value;
        this.render();
    }

    async toggleBool(key, value) {
        await this.saveSetting(key, value);
    }

    async toggleProxySwitch(value) {
        const label = document.querySelector('#page-settings .card-header .toggle-label');
        if (label) label.textContent = value ? I18n.t("settings.enabled") : I18n.t("settings.disabled");
        await this.saveSetting('proxy_enabled', value);
        showToast(I18n.t('settings.proxyToggleReload', {status: value ? I18n.t('common.on') : I18n.t('common.off')}));
        await this.reloadProxy();
    }

    async updateNumber(key, rawValue, type) {
        const value = type === 'int' ? parseInt(rawValue) : parseFloat(rawValue);
        if (isNaN(value)) return;
        await this.saveSetting(key, value);
    }

    async updateString(key, value) {
        await this.saveSetting(key, value);
    }

    async updateJson(key, rawValue) {
        try {
            const value = JSON.parse(rawValue);
            await this.saveSetting(key, value);
        } catch (e) {
            showToast(I18n.t("common.jsonError") + e.message, 'error');
        }
    }

    async saveSetting(key, value) {
        try {
            await API.put(`/settings/${encodeURIComponent(key)}`, { value });
            showToast(I18n.t('settings.settingUpdated', {key: key}));
            const s = this.settings.find(x => x.key === key);
            if (s) s.value = value;
        } catch (e) {
            console.error('Failed to save setting:', e);
            showToast(I18n.t("settings.saveFailedRetry"), 'error');
            this.loadSettings();
        }
    }

    editSetting(key) {
        const s = this.settings.find(x => x.key === key);
        if (!s) return;

        const displayVal = s.value_type === 'json' && typeof s.value === 'object'
            ? JSON.stringify(s.value, null, 2)
            : String(s.value);

        const modalHtml = `
            <div id="edit-setting-modal" class="modal" style="display:flex">
                <div class="modal-content">
                    <div class="modal-header">
                        <h3>${I18n.t('settings.editSetting')}${this.escHtml(s.key)}</h3>
                        <button class="modal-close" onclick="settingsPage.closeEditModal()">&times;</button>
                    </div>
                    <div class="modal-body">
                        <div class="form-group">
                            <label>${I18n.t('settings.keyName')}</label>
                            <input type="text" value="${this.escHtml(s.key)}" disabled class="form-input">
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('settings.descCol')}</label>
                            <input type="text" value="${this.escHtml(s.description || '')}" disabled class="form-input">
                        </div>
                        <div class="form-group">
                            <label>${I18n.t('settings.valueOf')}${this.escHtml(this.VALUE_TYPE_LABELS[s.value_type] || s.value_type)}）</label>
                            ${s.value_type === 'bool'
                                ? `<label class="toggle-switch">
                                       <input type="checkbox" id="edit-setting-value" ${s.value ? 'checked' : ''}>
                                       <span class="toggle-slider"></span>
                                   </label>`
                                : s.value_type === 'json'
                                ? `<textarea id="edit-setting-value" rows="8" class="form-input" style="font-family:monospace">${this.escHtml(displayVal)}</textarea>`
                                : `<input type="${s.value_type === 'int' || s.value_type === 'float' ? 'number' : 'text'}"
                                         id="edit-setting-value"
                                         value="${this.escHtml(String(s.value))}"
                                         step="${s.value_type === 'float' ? '0.01' : '1'}"
                                         class="form-input">`
                            }
                        </div>
                        <div class="form-actions">
                            <button class="btn-primary" onclick="settingsPage.submitEdit('${this.escHtml(s.key)}', '${s.value_type}')">${I18n.t('common.save')}</button>
                            <button class="btn-secondary" onclick="settingsPage.closeEditModal()">${I18n.t('common.cancel')}</button>
                        </div>
                    </div>
                </div>
            </div>`;

        const old = document.getElementById('edit-setting-modal');
        if (old) old.remove();

        document.body.insertAdjacentHTML('beforeend', modalHtml);
    }

    async submitEdit(key, valueType) {
        let value;
        const raw = document.getElementById('edit-setting-value');

        if (valueType === 'bool') {
            value = raw.checked;
        } else if (valueType === 'int') {
            value = parseInt(raw.value);
        } else if (valueType === 'float') {
            value = parseFloat(raw.value);
        } else if (valueType === 'json') {
            try {
                value = JSON.parse(raw.value);
            } catch (e) {
                showToast(I18n.t("settings.jsonError"), 'error');
                return;
            }
        } else {
            value = raw.value;
        }

        await this.saveSetting(key, value);
        this.closeEditModal();
    }

    closeEditModal() {
        const modal = document.getElementById('edit-setting-modal');
        if (modal) modal.remove();
    }

    async backup() {
        try {
            const result = await API.post('/settings/backup', {});
            showToast(I18n.t("settings.backupSuccess") + (result.backup || ''));
        } catch (e) {
            showToast(I18n.t('settings.backupFailed'), 'error');
        }
    }

    showRestoreDialog() {
        const modalHtml = `
            <div id="restore-modal" class="modal" style="display:flex">
                <div class="modal-content">
                    <div class="modal-header">
                        <h3>${I18n.t('settings.restoreTitle')}</h3>
                        <button class="modal-close" onclick="settingsPage.closeRestoreModal()">&times;</button>
                    </div>
                    <div class="modal-body">
                        <div class="form-group">
                            <label>${I18n.t('settings.backupPath')}</label>
                            <input type="text" id="restore-path" class="form-input" placeholder="${I18n.t('settings.backupPathPlaceholder')}">
                        </div>
                        <div class="form-actions">
                            <button class="btn-primary" onclick="settingsPage.restore()">${I18n.t('settings.restore')}</button>
                            <button class="btn-secondary" onclick="settingsPage.closeRestoreModal()">${I18n.t('common.cancel')}</button>
                        </div>
                    </div>
                </div>
            </div>`;

        const old = document.getElementById('restore-modal');
        if (old) old.remove();
        document.body.insertAdjacentHTML('beforeend', modalHtml);
    }

    async restore() {
        const path = document.getElementById('restore-path').value.trim();
        if (!path) {
            showToast(I18n.t("settings.enterBackupPath"), 'error');
            return;
        }
        try {
            await API.post('/settings/restore', { backup_path: path });
            showToast(I18n.t("settings.restoreSuccess"));
            this.closeRestoreModal();
        } catch (e) {
            showToast(I18n.t("settings.restoreFailed") + (e.message || ''), 'error');
        }
    }

    closeRestoreModal() {
        const modal = document.getElementById('restore-modal');
        if (modal) modal.remove();
    }

    ROUTING_STRATEGY_LABELS = {
        smart: I18n.t("settings.smartRouting"),
        priority: I18n.t("common.priority"),
        round_robin: I18n.t("settings.roundRobin"),
        least_latency: I18n.t("settings.leastLatency"),
        cost_first: I18n.t("settings.costFirst"),
    };

    VALUE_TYPE_LABELS = {
        string: I18n.t("settings.typeString"),
        int: I18n.t("settings.typeInt"),
        float: I18n.t("settings.typeFloat"),
        bool: I18n.t("settings.typeBool"),
        json: 'JSON',
    };

    escHtml(str) {
        if (!str) return '';
        return String(str)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');
    }
}

const settingsPage = new SettingsPage();
