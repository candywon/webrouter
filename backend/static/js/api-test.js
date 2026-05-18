/**
 * API 测试页 — 通过 Flask 后端转发请求到 wr-proxy
 * 支持指定模型和 auto/smart 智能模式
 */
const ApiTestPage = {
    members: [],

    load() {
        this.loadMembers();
    },

    async loadMembers() {
        try {
            const data = await API.get('/team/members/keys');
            this.members = data.members || [];
            this.populateKeySelector();
        } catch (e) {
            console.error('Failed to load members:', e);
        }
    },

    populateKeySelector() {
        const sel = document.getElementById('at-api-key');
        if (!sel) return;
        const enabled = this.members.filter(m => m.enabled && m.key);
        sel.innerHTML = '<option value="">— 选择成员 —</option>'
            + enabled.map(m => `<option value="${m.key}">${this.escHtml(m.name)} (${m.key.slice(0, 16)}…)</option>`).join('');
    },

    toggleModelMode() {
        const mode = document.querySelector('input[name="at-model-mode"]:checked').value;
        document.getElementById('at-model-specific-group').style.display = mode === 'specific' ? '' : 'none';
        document.getElementById('at-auto-mode-group').style.display = mode === 'auto' ? '' : 'none';
    },

    async sendRequest() {
        const apiKey = document.getElementById('at-api-key').value;
        if (!apiKey) return this.showAlert('请选择 API Key');

        const mode = document.querySelector('input[name="at-model-mode"]:checked').value;
        let model;
        if (mode === 'auto') {
            model = document.getElementById('at-auto-mode').value;
        } else {
            model = document.getElementById('at-model-name').value;
        }
        if (!model) return this.showAlert('请填写模型名称或选择 auto/smart');

        const userMsg = document.getElementById('at-message').value;

        await this.doSend(apiKey, model, userMsg);
    },

    async sendAutoRequest() {
        document.querySelector('input[name="at-model-mode"][value="auto"]').checked = true;
        document.getElementById('at-auto-mode').value = 'auto';
        this.toggleModelMode();
        await this.sendRequest();
    },

    async doSend(apiKey, model, userMsg) {
        const btn = document.getElementById('at-send-btn');
        btn.disabled = true;
        btn.textContent = '⏳ 发送中...';

        const card = document.getElementById('at-response-card');
        card.style.display = '';
        document.getElementById('at-response-content').textContent = '等待响应...';
        document.getElementById('at-response-content').style.color = '';
        document.getElementById('at-response-time').textContent = '';
        document.getElementById('at-response-model').textContent = '';
        document.getElementById('at-usage-info').textContent = '';
        document.getElementById('at-raw-response').textContent = '';

        const startTime = Date.now();
        const body = {
            api_key: apiKey,
            model: model,
            messages: [{ role: 'user', content: userMsg }],
            stream: false,
        };

        try {
            const result = await API.post('/settings/test-proxy', body);
            const elapsed = Date.now() - startTime;

            if (result.error) {
                document.getElementById('at-response-content').textContent = `请求失败:\n${result.error}`;
                document.getElementById('at-response-content').style.color = 'var(--danger)';
                document.getElementById('at-response-time').textContent = `${elapsed}ms`;
                return;
            }

            // 解析响应
            const content = result.choices?.[0]?.message?.content || '(空响应)';
            document.getElementById('at-response-content').textContent = content;
            document.getElementById('at-response-time').textContent = `${elapsed}ms`;
            document.getElementById('at-response-model').textContent = result.model || model;

            if (result.usage) {
                this.showUsage(result.usage);
            }

            document.getElementById('at-raw-response').textContent = JSON.stringify(result, null, 2);
        } catch (e) {
            const elapsed = Date.now() - startTime;
            document.getElementById('at-response-content').textContent = `请求失败: ${e.message}`;
            document.getElementById('at-response-content').style.color = 'var(--danger)';
            document.getElementById('at-response-time').textContent = `${elapsed}ms`;
        } finally {
            btn.disabled = false;
            btn.textContent = '🚀 发送请求';
        }
    },

    showUsage(usage) {
        const parts = [];
        if (usage.prompt_tokens) parts.push(`输入: ${usage.prompt_tokens} tokens`);
        if (usage.completion_tokens) parts.push(`输出: ${usage.completion_tokens} tokens`);
        if (usage.total_tokens) parts.push(`总计: ${usage.total_tokens} tokens`);
        if (usage.prompt_tokens_details?.cached_tokens) parts.push(`缓存命中: ${usage.prompt_tokens_details.cached_tokens} tokens`);
        document.getElementById('at-usage-info').textContent = parts.join('  │  ');
    },

    showAlert(msg) {
        if (typeof showToast === 'function') {
            showToast(msg, 'error');
        } else {
            alert(msg);
        }
    },

    escHtml(str) {
        if (!str) return '';
        return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    },
};
