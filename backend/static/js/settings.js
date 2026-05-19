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
    }

    async load() {
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

    async loadSettings() {
        try {
            const data = await API.get('/settings/all');
            this.settings = data.settings || [];
            this.buildCategories();
            this.render();
        } catch (e) {
            console.error('Failed to load settings:', e);
            const el = document.getElementById('settings-content');
            if (el) el.innerHTML = '<div class="empty-state"><p>加载失败，请刷新重试</p></div>';
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
            'general': '通用设置',
            'proxy': '代理配置',
            'monitor': '监控检测',
            'alert': '告警配置',
            'advanced': '高级设置',
        };

        let html = `
            <div class="page-header">
                <h2>⚙️ 系统设置</h2>
                <div>
                    <button class="btn-secondary" onclick="settingsPage.loadSettings()">🔄 刷新</button>
                </div>
            </div>

            ${this.renderGatewayCard()}

            ${this.renderSpecialCards()}

            <div class="card">
                <div class="card-header"><span class="card-title">全部设置</span></div>
                ${this.renderSettingsTable()}
            </div>

            <div class="card">
                <div class="card-header"><span class="card-title">数据备份</span></div>
                <div style="display:flex;gap:12px;padding:16px;">
                    <button class="btn-primary" onclick="settingsPage.backup()">📦 创建备份</button>
                    <button class="btn-secondary" onclick="settingsPage.showRestoreDialog()">📥 恢复备份</button>
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
                <span class="card-title">🚪 代理网关设置</span>
                <label class="toggle-switch">
                    <input type="checkbox" ${proxyOn ? 'checked' : ''}
                           onchange="settingsPage.toggleProxySwitch(this.checked)">
                    <span class="toggle-slider"></span>
                    <span class="toggle-label">${proxyOn ? '已开启' : '已关闭'}</span>
                </label>
            </div>
            <div style="padding:16px;">
                <p style="color:var(--text-muted);font-size:12px;margin-bottom:12px;">管理后台与 wr-proxy 通信地址，以及对外用户调用的 API 网关地址。</p>
                <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">
                    <div class="form-group">
                        <label>管理后台通信 URL</label>
                        <input type="text" id="gw-proxy-url" value="${this.escHtml(proxyUrl ? proxyUrl.value : '')}"
                               onchange="settingsPage.updateString('proxy_url', this.value)"
                               placeholder="http://localhost:5051">
                        <span style="font-size:11px;color:var(--text-muted);">Flask 内部连接 wr-proxy 的地址</span>
                    </div>
                    <div class="form-group">
                        <label>对外网关地址</label>
                        <input type="text" id="gw-gateway-url" value="${this.escHtml(gatewayUrl ? gatewayUrl.value : '')}"
                               onchange="settingsPage.updateString('gateway_url', this.value)"
                               placeholder="http://公网IP或域名:5051">
                        <span style="font-size:11px;color:var(--text-muted);">成员邀请邮件和文档中展示的 API 地址</span>
                    </div>
                    <div class="form-group">
                        <label>路由策略</label>
                        <select onchange="settingsPage.updateString('routing_strategy', this.value)">
                            ${['smart','priority','round_robin','least_latency','cost_first'].map(s =>
                                `<option value="${s}" ${routingStrategy && routingStrategy.value === s ? 'selected' : ''}>${this.ROUTING_STRATEGY_LABELS[s]}</option>`
                            ).join('')}
                    </div>
                    <div class="form-group">
                        <label>默认超时（秒）</label>
                        <input type="number" value="${defaultTimeout ? defaultTimeout.value : 60}" min="1"
                               onchange="settingsPage.updateNumber('default_timeout', this.value, 'int')">
                    </div>
                    <div class="form-group">
                        <label>最大降级次数</label>
                        <input type="number" value="${maxFailover ? maxFailover.value : 3}" min="0" max="10"
                               onchange="settingsPage.updateNumber('max_failover', this.value, 'int')">
                    </div>
                    <div class="form-group">
                        <label>最大重试次数</label>
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

        // wr-proxy 优化特性开关
        html += this.renderFeatureToggles();

        // 六维度复杂度配置
        html += this.renderComplexityConfig();

        // 日志清理周期
        if (logSetting) {
            html += `
            <div class="card">
                <div class="card-header"><span class="card-title">🗑️ 日志清理</span></div>
                <div style="padding:16px;display:flex;align-items:center;gap:12px;">
                    <label style="font-size:14px;">日志保留天数：</label>
                    <input type="number" id="log-retention-input" value="${logSetting.value}" min="1" max="365"
                           style="width:80px;padding:6px 10px;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);color:var(--text-primary);font-size:14px;">
                    <span style="color:var(--text-muted);font-size:13px;">天（当前：${logSetting.value} 天，每 10 分钟检查一次）</span>
                    <button class="btn-primary" onclick="settingsPage.saveLogRetention()" style="margin-left:auto;">保存</button>
                </div>
            </div>`;
        }

        // 厂商健康测试配置
        if (healthSetting) {
            const configs = Array.isArray(healthSetting.value) ? healthSetting.value : [];
            html += `
            <div class="card">
                <div class="card-header">
                    <span class="card-title">🔍 厂商健康测试配置</span>
                    <button class="btn-primary btn-sm" onclick="settingsPage.addHealthConfig()">+ 添加厂商</button>
                </div>
                <div style="padding:16px;">
                    <p style="color:var(--text-muted);font-size:12px;margin-bottom:12px;">直连 Provider 健康检测时使用，按 base_url 中的域名匹配测试端点和请求体。</p>
                    <table>
                        <thead><tr>
                            <th>厂商名称</th><th>域名匹配</th><th>测试端点</th><th>测试请求体</th><th>操作</th>
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
                                <td><button class="btn-icon" onclick="settingsPage.removeHealthConfig(${i})" title="删除">🗑️</button></td>
                            </tr>`).join('')}
                        </tbody>
                    </table>
                    <button class="btn-primary" onclick="settingsPage.saveHealthConfigs()" style="margin-top:12px;">💾 保存全部</button>
                    <button class="btn-secondary" onclick="settingsPage.resetHealthConfigs()" style="margin-top:12px;margin-left:8px;">↩️ 恢复默认</button>
                </div>
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
                <div class="card-header"><span class="card-title">🔔 告警通知配置</span></div>
                <div style="padding:16px;">
                    <p style="color:var(--text-muted);font-size:12px;margin-bottom:16px;">配置微信和邮件告警通道，在"告警规则"页面选择通道后自动使用。</p>
                    <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">
                        <div class="form-group" style="grid-column:span 2;">
                            <label>Server酱 SendKey（微信推送）</label>
                            <input type="text" id="alert-wechat-sendkey" value="${this.escHtml(wechatSendkey)}" placeholder="在 https://sct.ftqq.com/ 获取">
                        </div>
                        <div class="form-group">
                            <label>SMTP 服务器地址</label>
                            <input type="text" id="alert-smtp-host" value="${this.escHtml(smtpHost)}" placeholder="如 smtp.gmail.com">
                        </div>
                        <div class="form-group">
                            <label>SMTP 端口</label>
                            <input type="number" id="alert-smtp-port" value="${smtpPort}" min="1" max="65535">
                        </div>
                        <div class="form-group">
                            <label>SMTP 用户名</label>
                            <input type="text" id="alert-smtp-user" value="${this.escHtml(smtpUser)}" placeholder="登录邮箱">
                        </div>
                        <div class="form-group">
                            <label>SMTP 密码</label>
                            <input type="password" id="alert-smtp-password" value="${this.escHtml(smtpPass)}" placeholder="邮箱密码或应用专用密码（QQ 邮箱请使用授权码）">
                        </div>
                        <div class="form-group">
                            <label>发件人地址（留空则用用户名）</label>
                            <input type="text" id="alert-smtp-from" value="${this.escHtml(smtpFrom)}" placeholder="发件人邮箱">
                        </div>
                        <div class="form-group">
                            <label>收件人地址（逗号分隔）</label>
                            <input type="text" id="alert-email-to" value="${this.escHtml(emailTo)}" placeholder="admin@example.com">
                        </div>
                    </div>
                    <button class="btn-primary" onclick="settingsPage.saveAlertNotify()" style="margin-top:12px;">💾 保存告警配置</button>
                    <button class="btn-secondary" onclick="settingsPage.sendTestEmail()" style="margin-top:12px;margin-left:8px;">📧 发送测试邮件</button>
                </div>
            </div>`;
        }

        return html;
    }

    // 渲染 wr-proxy 优化特性开关卡片
    renderFeatureToggles() {
        const featDefs = [
            {
                key: 'feature_dynamic_content_last',
                title: '📌 动态内容后置',
                shortDesc: '将 user 消息中的动态内容（URL、时间、随机数等）移到末尾，提升 prompt cache 命中率。',
                detail: '开启后 wr-proxy 会对请求 body 中的 messages 数组重新排序：把包含 URL、日期、数字等动态内容的 message 移到同 role 组的最后，使 prompt 前缀尽可能保持静态，从而提升上游 prompt cache 命中率。适合固定 system prompt + 动态用户输入的场景。',
                icon: '🔀',
            },
            {
                key: 'feature_token_compression',
                title: '🗜️ Token 压缩（RTK）',
                shortDesc: '对系统提示词和长上下文进行压缩预处理，减少输入 token 数量。',
                detail: '在请求发送到上游之前，先通过一次轻量模型（如 qwen-turbo）对长文本做摘要，减少输入 token 数量。适用于 system prompt 很长的场景（>4000 tokens），可显著降低调用成本，但会损失少量上下文精度。开启后需要额外配置压缩模型（见下方设置）。',
                icon: '📦',
            },
            {
                key: 'feature_session_compression',
                title: '🔄 会话压缩',
                shortDesc: '对多轮对话的历史消息进行压缩，将早期消息合并为摘要。',
                detail: '当对话轮数超过阈值时，将早期消息合并为摘要，减少后续请求的上下文长度。适用于长对话场景（如客服、助教），可将数十轮对话压缩为几轮摘要。注意：压缩会丢失部分细节，适合对上下文精度要求不高的场景。开启后需要配置压缩阈值和模型（见下方设置）。',
                icon: '📉',
            },
        ];

        const dbSettings = {};
        this.settings.forEach(s => { dbSettings[s.key] = s; });

        return `
        <div class="card" style="border-left:3px solid var(--accent);">
            <div class="card-header">
                <span class="card-title">⚡ wr-proxy 优化特性</span>
                <button class="btn-primary btn-sm" onclick="settingsPage.reloadProxy()" id="btn-reload-proxy">🔄 Reload wr-proxy</button>
            </div>
            <div style="padding:16px;">
                <p style="color:var(--text-muted);font-size:12px;margin-bottom:16px;">以下特性为 wr-proxy 高级优化功能，开启后会在请求转发到上游之前自动处理请求体，以优化 token 使用和 cache 命中率。修改开关后需要点击 Reload 按钮生效。</p>
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
                        <span class="state-label" style="margin-left:auto;font-size:12px;color:${enabled ? 'var(--success)' : 'var(--text-muted)'};">${enabled ? '已开启' : '已关闭'}</span>
                    </div>
                    <p style="color:var(--text-muted);font-size:13px;margin:0 0 6px 0;">${this.escHtml(feat.shortDesc)}</p>
                    <details style="color:var(--text-secondary);font-size:12px;margin:0;">
                        <summary style="cursor:pointer;color:var(--accent);">详细说明 ▾</summary>
                        <p style="margin:6px 0 0 0;line-height:1.6;">${this.escHtml(feat.detail)}</p>
                        ${desc !== feat.shortDesc ? `<p style="margin:6px 0 0 0;color:var(--text-muted);font-size:11px;">💡 数据库中保存的说明：${this.escHtml(desc)}</p>` : ''}
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
            showToast('初始化失败: ' + (e.message || ''), 'error');
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
            <span style="font-weight:600;color:var(--accent);">🎯 分级阈值</span>
            <span style="color:var(--text-muted);font-size:13px;">
                <input type="number" step="0.01" min="0" max="1" value="${tier.simple_max}"
                    onchange="settingsPage.updateComplexityTier('simple_max', this.value)"
                    style="width:64px;padding:4px 8px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:13px;text-align:center;">
                <span style="margin:0 4px;">简单→经济</span>
                &nbsp;│&nbsp;
                <input type="number" step="0.01" min="0" max="1" value="${tier.moderate_max}"
                    onchange="settingsPage.updateComplexityTier('moderate_max', this.value)"
                    style="width:64px;padding:4px 8px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:13px;text-align:center;">
                <span style="margin:0 4px;">中等→标准</span>
                &nbsp;≥&nbsp;<span style="font-weight:500;">${tier.moderate_max}</span>&nbsp;→ premium
            </span>
        </div>`;

        // 六维度定义
        const dims = [
            {
                key: 'input_length', icon: '📏', title: '输入长度',
                desc: '按请求消息的总字符数评分，输入越长意味着处理难度越大、需要更强模型的可能性越高。',
                type: 'levels', unit: '字符', field: 'max_chars',
            },
            {
                key: 'multi_turn', icon: '💬', title: '多轮对话',
                desc: '按对话轮数评分，轮数越多意味着需要更多上下文理解能力，更适合强模型。',
                type: 'levels', unit: '轮', field: 'max_msgs',
            },
            {
                key: 'code_detection', icon: '💻', title: '代码检测',
                desc: '检测消息中是否包含代码特征（代码块、函数定义、类等），编程任务通常需要更强的推理能力。',
                type: 'keywords', keywordsField: 'keywords', scoreField: 'score',
            },
            {
                key: 'tools_detection', icon: '🔧', title: '工具调用',
                desc: '检测请求中是否包含 tools 或 functions 字段，使用工具调用通常意味着更复杂的任务规划，需要更强模型。',
                type: 'scores', toolsScoreField: 'tools_score', functionsScoreField: 'functions_score',
            },
            {
                key: 'reasoning_keywords', icon: '🧠', title: '推理关键词',
                desc: '检测最后一条用户消息是否包含推理/分析类关键词（如"分析""原理""推理"等），命中即意味着任务需要更强推理能力。',
                type: 'keywords', keywordsField: 'keywords', scoreField: 'score',
            },
            {
                key: 'system_prompt', icon: '📋', title: '系统提示词',
                desc: '检测 system prompt 的长度，过长的系统提示词通常包含复杂的指令或角色设定，需要更强模型来遵循。',
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
        const reasonDet = cfg.reasoning_keywords || { enabled: true, score: 0.12, keywords: ['分析', '推理', '证明', '计算', '推导', 'explain', 'analyze', 'reason', 'prove', 'calculate', 'derive', 'compare', 'evaluate', 'critique', '为什么', '原因', '原理', '逻辑', '步骤', '方案', '策略', '设计'] };
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
                        <th style="padding:4px 8px;text-align:left;font-weight:500;">条件</th>
                        <th style="padding:4px 8px;text-align:center;font-weight:500;">得分</th>
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
                        <div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;">关键词（逗号分隔）</div>
                        <input type="text" value="${this.escHtml(kw)}"
                            onchange="settingsPage.updateComplexityKeywords('${dim.key}', this.value)"
                            style="width:100%;padding:5px 8px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:12px;font-family:monospace;">
                    </div>
                    <div style="min-width:80px;">
                        <div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;">命中得分</div>
                        <input type="number" step="0.01" min="0" max="1" value="${dc[dim.scoreField]}"
                            onchange="settingsPage.updateComplexityField('${dim.key}', '${dim.scoreField}', this.value)"
                            style="width:64px;padding:5px 6px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:13px;text-align:center;">
                    </div>
                </div>`;
            } else if (dim.type === 'scores') {
                contentHtml = `<div style="display:flex;gap:20px;">
                    <div>
                        <div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;">tools 得分</div>
                        <input type="number" step="0.01" min="0" max="1" value="${dc.tools_score}"
                            onchange="settingsPage.updateComplexityField('tools_detection', 'tools_score', this.value)"
                            style="width:64px;padding:5px 6px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:13px;text-align:center;">
                    </div>
                    <div>
                        <div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;">functions 得分</div>
                        <input type="number" step="0.01" min="0" max="1" value="${dc.functions_score}"
                            onchange="settingsPage.updateComplexityField('tools_detection', 'functions_score', this.value)"
                            style="width:64px;padding:5px 6px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:13px;text-align:center;">
                    </div>
                </div>`;
            } else if (dim.type === 'threshold') {
                contentHtml = `<div style="display:flex;gap:20px;">
                    <div>
                        <div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;">字符阈值</div>
                        <input type="number" min="0" value="${dc.threshold_chars}"
                            onchange="settingsPage.updateComplexityField('system_prompt', 'threshold_chars', this.value)"
                            style="width:80px;padding:5px 6px;background:var(--bg-card);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);font-size:13px;text-align:center;">
                    </div>
                    <div>
                        <div style="font-size:12px;color:var(--text-muted);margin-bottom:4px;">命中得分</div>
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
                        <span class="state-label" style="font-size:12px;color:${accentColor};margin-left:auto;">${on ? '已启用' : '已禁用'}</span>
                    </div>
                    ${on ? contentHtml : ''}
                    <details style="margin-top:6px;color:var(--text-muted);font-size:12px;">
                        <summary style="cursor:pointer;color:var(--accent);">说明 ▾</summary>
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
                <span class="card-title">🎯 智能模型选择 — 复杂度评估配置</span>
                <div>
                    <button class="btn-secondary btn-sm" onclick="settingsPage.resetComplexityConfig()" style="margin-right:8px;">↩️ 恢复默认</button>
                    <button class="btn-primary btn-sm" onclick="settingsPage.saveComplexityConfig()" style="margin-right:8px;">💾 保存配置</button>
                    <button class="btn-secondary btn-sm" onclick="settingsPage.reloadProxy()">🔄 Reload</button>
                </div>
            </div>
            <div style="padding:16px;">
                <p style="color:var(--text-muted);font-size:12px;margin-bottom:12px;">
                    六维度独立评分，总分决定模型分级。修改配置后需点击 <code>Reload wr-proxy</code> 生效。
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
        showToast('复杂度配置已保存，请点击 Reload wr-proxy 生效');
    }

    async resetComplexityConfig() {
        if (!confirm('确定恢复默认复杂度配置？当前自定义设置将被覆盖。')) return;
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
            "reasoning_keywords": { "enabled": true, "score": 0.12, "keywords": ["分析", "推理", "证明", "计算", "推导", "explain", "analyze", "reason", "prove", "calculate", "derive", "compare", "evaluate", "critique", "为什么", "原因", "原理", "逻辑", "步骤", "方案", "策略", "设计"] },
            "system_prompt": { "enabled": true, "threshold_chars": 500, "score": 0.08 },
        };
        await this.saveSetting('smart_complexity_config', defaults);
        await this.loadSettings();
        showToast('已恢复默认复杂度配置');
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
            label.textContent = value ? '已开启' : '已关闭';
            label.style.color = value ? 'var(--success)' : 'var(--text-muted)';
        }
        await this.toggleFeature(key, value);
    }

    async reloadProxy() {
        const btn = document.getElementById('btn-reload-proxy');
        if (btn) { btn.disabled = true; btn.textContent = '⏳ 重载中...'; }
        try {
            const result = await API.post('/settings/reload-proxy', {});
            if (result.success) {
                showToast('🔄 ' + result.message + (result.detail ? ' — ' + result.detail : ''));
            } else {
                showToast('重载失败: ' + result.message, 'error');
            }
        } catch (e) {
            showToast('重载失败: ' + (e.message || '网络错误'), 'error');
        } finally {
            if (btn) { btn.disabled = false; btn.textContent = '🔄 Reload wr-proxy'; }
        }
    }

    async saveLogRetention() {
        const input = document.getElementById('log-retention-input');
        const value = parseInt(input.value);
        if (isNaN(value) || value < 1) {
            showToast('保留天数必须是正整数', 'error');
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
        if (!confirm('确定恢复默认厂商测试配置？')) return;
        const defaults = [
            { domain: 'api.openai.com', name: 'OpenAI', endpoint: '/v1/chat/completions', body: '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}],"max_tokens":1}' },
            { domain: 'api.anthropic.com', name: 'Anthropic', endpoint: '/v1/messages', body: '{"model":"claude-3-haiku-20240307","messages":[{"role":"user","content":"hi"}],"max_tokens":1}' },
            { domain: 'api.deepseek.com', name: 'DeepSeek', endpoint: '/v1/chat/completions', body: '{"model":"deepseek-chat","messages":[{"role":"user","content":"hi"}],"max_tokens":1}' },
            { domain: 'dashscope.aliyuncs.com', name: '通义千问', endpoint: '/compatible-mode/v1/chat/completions', body: '{"model":"qwen-turbo","messages":[{"role":"user","content":"hi"}],"max_tokens":1}' },
            { domain: 'open.bigmodel.cn', name: '智谱', endpoint: '/v4/chat/completions', body: '{"model":"glm-4-flash","messages":[{"role":"user","content":"hi"}],"max_tokens":1}' },
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
            showToast('至少配置微信 SendKey 或 SMTP 地址之一', 'error');
            return;
        }
        if (smtpHost && !emailTo) {
            showToast('配置了 SMTP 地址时必须填写 收件人', 'error');
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
        showToast('告警通知配置已保存');
    }

    async sendTestEmail() {
        // 先保存当前表单值
        const smtpHost = document.getElementById('alert-smtp-host').value.trim();
        const emailTo = document.getElementById('alert-email-to').value.trim();
        const smtpUser = document.getElementById('alert-smtp-user').value.trim();
        const smtpPass = document.getElementById('alert-smtp-password').value;

        if (!smtpHost || !emailTo || !smtpUser || !smtpPass) {
            showToast('请先填写 SMTP 配置（服务器地址、用户名、密码、收件人）', 'error');
            return;
        }

        // 先保存配置再发送测试邮件
        await this.saveAlertNotify();

        try {
            showToast('正在发送测试邮件...');
            const result = await API.post('/settings/test-email', {});
            if (result.success) {
                showToast('📧 ' + result.message);
            } else {
                showToast('发送失败: ' + result.message, 'error');
            }
        } catch (e) {
            showToast('发送失败: ' + (e.message || '网络错误'), 'error');
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
            s.key !== 'smart_complexity_config'
        );

        if (filtered.length === 0) {
            return '<div class="empty-state"><p>暂无设置项</p></div>';
        }

        const categoryLabels = {
            'general': '📋 通用',
            'proxy': '🔌 代理',
            'monitor': '📊 监控',
            'alert': '🔔 告警',
            'advanced': '⚙️ 高级',
        };

        let html = '<div class="table-wrap"><table><thead><tr>'
            + '<th>键名</th><th>值</th><th>类型</th><th>分类</th><th>说明</th><th>操作</th>'
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
                            ? `<button class="btn-sm" onclick="settingsPage.editSetting('${this.escHtml(s.key)}')">✏️ 编辑</button>`
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
                               onchange="this.closest('label').querySelector('.toggle-label').textContent=this.checked?'开启':'关闭';settingsPage.toggleBool('${key}',this.checked)"
                               ${setting.editable ? '' : 'disabled'}>
                        <span class="toggle-slider"></span>
                        <span class="toggle-label">${val ? '开启' : '关闭'}</span>
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
        if (label) label.textContent = value ? '已开启' : '已关闭';
        await this.saveSetting('proxy_enabled', value);
        showToast(`代理网关已${value ? '开启' : '关闭'}，正在重载 wr-proxy...`);
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
            showToast('JSON 格式错误: ' + e.message, 'error');
        }
    }

    async saveSetting(key, value) {
        try {
            await API.put(`/settings/${encodeURIComponent(key)}`, { value });
            showToast(`设置 ${key} 已更新`);
            const s = this.settings.find(x => x.key === key);
            if (s) s.value = value;
        } catch (e) {
            console.error('Failed to save setting:', e);
            showToast('保存失败，请重试', 'error');
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
                        <h3>编辑设置: ${this.escHtml(s.key)}</h3>
                        <button class="modal-close" onclick="settingsPage.closeEditModal()">&times;</button>
                    </div>
                    <div class="modal-body">
                        <div class="form-group">
                            <label>键名</label>
                            <input type="text" value="${this.escHtml(s.key)}" disabled class="form-input">
                        </div>
                        <div class="form-group">
                            <label>说明</label>
                            <input type="text" value="${this.escHtml(s.description || '')}" disabled class="form-input">
                        </div>
                        <div class="form-group">
                            <label>值（${this.escHtml(this.VALUE_TYPE_LABELS[s.value_type] || s.value_type)}）</label>
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
                            <button class="btn-primary" onclick="settingsPage.submitEdit('${this.escHtml(s.key)}', '${s.value_type}')">保存</button>
                            <button class="btn-secondary" onclick="settingsPage.closeEditModal()">取消</button>
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
                showToast('JSON 格式错误', 'error');
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
            showToast('备份成功: ' + (result.backup || ''));
        } catch (e) {
            showToast('备份失败', 'error');
        }
    }

    showRestoreDialog() {
        const modalHtml = `
            <div id="restore-modal" class="modal" style="display:flex">
                <div class="modal-content">
                    <div class="modal-header">
                        <h3>恢复备份</h3>
                        <button class="modal-close" onclick="settingsPage.closeRestoreModal()">&times;</button>
                    </div>
                    <div class="modal-body">
                        <div class="form-group">
                            <label>备份文件路径</label>
                            <input type="text" id="restore-path" class="form-input" placeholder="如: /path/to/webrouter.db.backup_20250515_120000">
                        </div>
                        <div class="form-actions">
                            <button class="btn-primary" onclick="settingsPage.restore()">恢复</button>
                            <button class="btn-secondary" onclick="settingsPage.closeRestoreModal()">取消</button>
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
            showToast('请输入备份路径', 'error');
            return;
        }
        try {
            await API.post('/settings/restore', { backup_path: path });
            showToast('恢复成功，请刷新页面');
            this.closeRestoreModal();
        } catch (e) {
            showToast('恢复失败: ' + (e.message || ''), 'error');
        }
    }

    closeRestoreModal() {
        const modal = document.getElementById('restore-modal');
        if (modal) modal.remove();
    }

    ROUTING_STRATEGY_LABELS = {
        smart: '智能调度',
        priority: '优先级',
        round_robin: '轮询',
        least_latency: '最低延迟',
        cost_first: '成本优先',
    };

    VALUE_TYPE_LABELS = {
        string: '字符串',
        int: '整数',
        float: '浮点数',
        bool: '布尔',
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
