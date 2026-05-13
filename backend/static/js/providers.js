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
                '<div class="error-msg">加载失败，请刷新重试</div>';
        }
    }

    bindEvents() {
        // 事件绑定由 render() 中的 onclick 内联处理
    }

    render() {
        const container = document.getElementById('page-content');
        if (!container) return;

        const statusIcon = {
            'healthy': '🟢',
            'warning': '🟡',
            'dead': '🔴',
            'disabled': '⏸',
            'rate_limited': '🟡',
            'auth_failed': '🔴',
            'timeout': '🟡',
            'unchecked': '⚪',
            'unhealthy': '🟠',
            'unknown': '⚪',
        };

        const typeLabel = {
            'direct': '直连官方',
            'aggregate': '聚合平台',
            'newapi': 'New-API',
            'oneapi': 'One-API',
            'litellm': 'LiteLLM',
            'custom': '自定义',
        };

        let html = `
            <div class="page-header">
                <h2>🔌 数据源管理</h2>
                <button class="btn-primary" onclick="providersPage.showAddForm()">+ 添加数据源</button>
            </div>
            <div class="provider-list">
        `;

        if (this.providers.length === 0) {
            html += `
                <div class="empty-state">
                    <p>还没有注册任何数据源</p>
                    <p class="hint">点击"添加数据源"开始管理你的 API 资源</p>
                </div>
            `;
        } else {
            for (const p of this.providers) {
                const icon = statusIcon[p.status] || '⚪';
                const type = typeLabel[p.type] || p.type;
                const latency = p.last_latency_ms != null ? `${p.last_latency_ms}ms` : '-';
                const checked = p.last_check_at ? this.formatTime(p.last_check_at) : '未检测';

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
                        <span class="provider-checked">检测于 ${checked}</span>
                    </div>
                    ${p.api_key_masked ? `<div class="provider-key">Key: ${this.escHtml(p.api_key_masked)}</div>` : ''}
                    ${p.last_error ? `<div class="provider-error">错误: ${this.escHtml(p.last_error)}</div>` : ''}
                    <div class="provider-actions">
                        <button class="btn-sm" onclick="providersPage.checkOne(${p.id})">🔍 检测</button>
                        <button class="btn-sm" onclick="providersPage.editProvider(${p.id})">✏️ 编辑</button>
                        <button class="btn-sm btn-danger" onclick="providersPage.deleteProvider(${p.id})">🗑️ 删除</button>
                    </div>
                </div>
                `;
            }
        }

        html += `
            </div>
            <div class="provider-actions-bar">
                <button class="btn-secondary" onclick="providersPage.checkAll()">🔍 全量检测</button>
            </div>

            <!-- 添加/编辑表单（隐藏） -->
            <div id="provider-form-modal" class="modal" style="display:none">
                <div class="modal-content">
                    <div class="modal-header">
                        <h3 id="form-title">添加数据源</h3>
                        <button class="modal-close" onclick="providersPage.hideForm()">&times;</button>
                    </div>
                    <div class="modal-body">
                        <form id="provider-form">
                            <div class="form-group">
                                <label>类型</label>
                                <select id="pf-type" onchange="providersPage.onTypeChange()">
                                    <option value="direct">🔌 直连官方</option>
                                    <option value="aggregate">🔀 聚合平台</option>
                                    <option value="newapi">🏗️ 自建 New-API</option>
                                    <option value="oneapi">🏗️ 自建 One-API</option>
                                    <option value="litellm">🦙 LiteLLM 代理</option>
                                    <option value="custom">⚙️ 自定义网关</option>
                                </select>
                            </div>
                            <div class="form-group">
                                <label>名称 *</label>
                                <input type="text" id="pf-name" required placeholder="如: OpenAI 官方">
                            </div>
                            <div class="form-group">
                                <label>Base URL *</label>
                                <input type="text" id="pf-base-url" required placeholder="如: https://api.openai.com">
                                <button type="button" class="btn-sm" onclick="providersPage.autoDetect()" style="margin-top:4px">🔍 自动检测类型</button>
                            </div>
                            <div class="form-group" id="pf-api-key-group">
                                <label>API Key</label>
                                <input type="password" id="pf-api-key" placeholder="sk-xxx">
                            </div>
                            <div class="form-group" id="pf-admin-token-group" style="display:none">
                                <label>Admin Token</label>
                                <input type="password" id="pf-admin-token" placeholder="New-API/One-API 管理令牌">
                            </div>
                            <div class="form-group" id="pf-db-uri-group" style="display:none">
                                <label>数据库连接串</label>
                                <input type="text" id="pf-db-uri" placeholder="如: sqlite:///data/new-api.db">
                            </div>
                            <div class="form-group" id="pf-master-key-group" style="display:none">
                                <label>Master Key</label>
                                <input type="password" id="pf-master-key" placeholder="LiteLLM Master Key">
                            </div>
                            <div class="form-group" id="pf-health-endpoint-group" style="display:none">
                                <label>健康检测端点</label>
                                <input type="text" id="pf-health-endpoint" placeholder="如: /health">
                            </div>
                            <div class="form-group">
                                <label>备注</label>
                                <textarea id="pf-notes" rows="2" placeholder="可选"></textarea>
                            </div>
                            <div class="form-actions">
                                <button type="submit" class="btn-primary">保存</button>
                                <button type="button" class="btn-secondary" onclick="providersPage.hideForm()">取消</button>
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
        document.getElementById('pf-admin-token-group').style.display =
            (type === 'newapi' || type === 'oneapi') ? '' : 'none';
        document.getElementById('pf-db-uri-group').style.display =
            (type === 'newapi' || type === 'oneapi') ? '' : 'none';
        document.getElementById('pf-master-key-group').style.display =
            (type === 'litellm') ? '' : 'none';
        document.getElementById('pf-health-endpoint-group').style.display =
            (type === 'custom') ? '' : 'none';
    }

    showAddForm() {
        this.editingId = null;
        document.getElementById('form-title').textContent = '添加数据源';
        document.getElementById('provider-form').reset();
        document.getElementById('pf-type').value = 'direct';
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
        document.getElementById('form-title').textContent = '编辑数据源';
        document.getElementById('pf-type').value = p.type;
        document.getElementById('pf-name').value = p.name;
        document.getElementById('pf-base-url').value = p.base_url;
        document.getElementById('pf-notes').value = p.notes || '';
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
        const data = {
            type,
            name: document.getElementById('pf-name').value.trim(),
            base_url: document.getElementById('pf-base-url').value.trim(),
            api_key: document.getElementById('pf-api-key').value.trim(),
            notes: document.getElementById('pf-notes').value.trim(),
        };

        if (type === 'newapi' || type === 'oneapi') {
            data.admin_token = document.getElementById('pf-admin-token').value.trim();
            data.db_uri = document.getElementById('pf-db-uri').value.trim();
        }
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
            alert('保存失败: ' + (e.message || '未知错误'));
        }
    }

    async checkOne(id) {
        try {
            const result = await API.post(`/providers/${id}/check`);
            alert(`${result.name}: ${result.status} (${result.latency_ms || 0}ms)`);
            await this.loadProviders();
        } catch (e) {
            alert('检测失败: ' + (e.message || '未知错误'));
        }
    }

    async checkAll() {
        try {
            const data = await API.post('/providers/check_all');
            alert(`检测完成: ${data.total}个数据源`);
            await this.loadProviders();
        } catch (e) {
            alert('全量检测失败: ' + (e.message || '未知错误'));
        }
    }

    async deleteProvider(id) {
        const p = this.providers.find(x => x.id === id);
        if (!p) return;
        if (!confirm(`确定删除数据源 "${p.name}" 吗？`)) return;

        try {
            await API.del(`/providers/${id}`);
            await this.loadProviders();
        } catch (e) {
            alert('删除失败: ' + (e.message || '未知错误'));
        }
    }

    async autoDetect() {
        const baseUrl = document.getElementById('pf-base-url').value.trim();
        if (!baseUrl) {
            alert('请先输入 Base URL');
            return;
        }

        try {
            const data = await API.post('/providers/detect', { base_url: baseUrl });
            if (data.detected_type) {
                document.getElementById('pf-type').value = data.detected_type;
                this.onTypeChange();
                alert(`检测到类型: ${data.type_config?.label || data.detected_type}`);
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
