/**
 * 模型定价管理页面 JS
 */
class PricingPage {
    constructor() {
        this.pricing = [];
        this.vendors = [];
        this.filterVendor = '';
        this.editingModel = null;
    }

    async load() {
        await Promise.all([this.loadPricing(), this.loadVendors()]);
    }

    async loadPricing() {
        try {
            let url = '/pricing/';
            if (this.filterVendor) url += '?vendor=' + encodeURIComponent(this.filterVendor);
            const data = await API.get(url);
            this.pricing = data.pricing || [];
            this.render();
        } catch (e) {
            console.error('Failed to load pricing:', e);
        }
    }

    async loadVendors() {
        try {
            const data = await API.get('/pricing/vendors');
            this.vendors = data.vendors || [];
            this.renderVendors();
        } catch (e) {
            console.error('Failed to load vendors:', e);
        }
    }

    // 分/千token → 元/百万token（×10）
    priceToYuan(centsPerK) {
        if (centsPerK == null) return '¥0';
        return '¥' + (centsPerK * 10).toFixed(2);
    }

    // 元/百万token → 分/千token（÷10）
    yuanToCents(yuan) {
        if (!yuan && yuan !== 0) return 0;
        return parseFloat((parseFloat(yuan) / 10).toFixed(4));
    }

    render() {
        const container = document.getElementById('pricing-content');
        if (!container) return;

        const vendorColor = {
            'openai': 'vendor-openai', 'anthropic': 'vendor-anthropic',
            'google': 'vendor-google', 'deepseek': 'vendor-deepseek',
            'qwen': 'vendor-qwen', 'zhipu': 'vendor-zhipu',
            'moonshot': 'vendor-moonshot',
        };

        let html = '';
        if (this.pricing.length === 0) {
            html = `<div class="empty-state"><p>暂无定价数据</p><p class="hint">点击"添加定价"配置模型价格</p></div>`;
        } else {
            html += `<div class="table-wrap"><table>
                <thead><tr>
                    <th>模型</th><th>厂商</th><th>输入价格</th><th>输出价格</th><th>默认</th><th>备注</th><th>操作</th>
                </tr></thead><tbody>`;

            for (const p of this.pricing) {
                const vendorCls = vendorColor[p.vendor] || 'vendor-other';
                html += `
                <tr>
                    <td><code class="model-name">${this.escHtml(p.model)}</code></td>
                    <td><span class="badge ${vendorCls}">${this.escHtml(p.vendor || 'other')}</span></td>
                    <td>${this.priceToYuan(p.input_price)}/M</td>
                    <td>${this.priceToYuan(p.output_price)}/M</td>
                    <td>${p.is_default ? '<span class="badge badge-healthy">默认</span>' : ''}</td>
                    <td class="pricing-notes">${this.escHtml(p.notes || '-')}</td>
                    <td>
                        <button class="btn-sm" onclick="pricingPage.editPricing('${this.escHtml(p.model)}')">✏️</button>
                        ${!p.is_default ? `<button class="btn-sm btn-danger" onclick="pricingPage.deletePricing('${this.escHtml(p.model)}')">🗑️</button>` : ''}
                    </td>
                </tr>`;
            }
            html += `</tbody></table></div>`;
        }
        container.innerHTML = html;
    }

    renderVendors() {
        const select = document.getElementById('vendor-filter');
        if (!select) return;

        let opts = '<option value="">全部厂商</option>';
        for (const v of this.vendors) {
            const selected = this.filterVendor === v.name ? 'selected' : '';
            opts += `<option value="${this.escHtml(v.name)}" ${selected}>${this.escHtml(v.name)} (${v.model_count})</option>`;
        }
        select.innerHTML = opts;
    }

    onVendorFilter() {
        this.filterVendor = document.getElementById('vendor-filter').value;
        this.loadPricing();
    }

    showAddForm() {
        this.editingModel = null;
        document.getElementById('pricing-form-title').textContent = '添加模型定价';
        document.getElementById('pricing-form').reset();
        document.getElementById('pf-model').value = '';
        document.getElementById('pf-model').disabled = false;
        document.getElementById('pf-input-price').value = '';
        document.getElementById('pf-output-price').value = '';
        document.getElementById('pf-vendor').value = 'other';
        document.getElementById('pf-is-default').checked = false;
        document.getElementById('pf-notes').value = '';
        document.getElementById('pricing-form-modal').style.display = 'flex';

        const form = document.getElementById('pricing-form');
        form.onsubmit = (e) => { e.preventDefault(); this.submitForm(); };
    }

    async editPricing(modelName) {
        const p = this.pricing.find(x => x.model === modelName);
        if (!p) return;

        this.editingModel = modelName;
        document.getElementById('pricing-form-title').textContent = '编辑定价';
        document.getElementById('pf-model').value = p.model;
        document.getElementById('pf-model').disabled = true;
        // 分/千token → 元/百万token 显示
        document.getElementById('pf-input-price').value = (p.input_price * 10).toFixed(4);
        document.getElementById('pf-output-price').value = (p.output_price * 10).toFixed(4);
        document.getElementById('pf-vendor').value = p.vendor || 'other';
        document.getElementById('pf-is-default').checked = p.is_default;
        document.getElementById('pf-notes').value = p.notes || '';
        document.getElementById('pricing-form-modal').style.display = 'flex';

        const form = document.getElementById('pricing-form');
        form.onsubmit = (e) => { e.preventDefault(); this.submitForm(); };
    }

    hideForm() {
        document.getElementById('pricing-form-modal').style.display = 'none';
        this.editingModel = null;
    }

    async submitForm() {
        const inputYuan = parseFloat(document.getElementById('pf-input-price').value) || 0;
        const outputYuan = parseFloat(document.getElementById('pf-output-price').value) || 0;
        const data = {
            model: document.getElementById('pf-model').value.trim(),
            input_price: this.yuanToCents(inputYuan),   // 元/百万token → 分/千token
            output_price: this.yuanToCents(outputYuan),
            vendor: document.getElementById('pf-vendor').value,
            is_default: document.getElementById('pf-is-default').checked,
            notes: document.getElementById('pf-notes').value.trim(),
        };

        if (!data.model) { showToast('模型名称不能为空'); return; }

        try {
            if (this.editingModel) {
                await API.put(`/pricing/${encodeURIComponent(this.editingModel)}`, data);
            } else {
                await API.post('/pricing/', data);
            }
            this.hideForm();
            await this.load();
            showToast('定价保存成功');
        } catch (e) {
            showToast('保存失败: ' + (e.message || '未知错误'));
        }
    }

    async deletePricing(modelName) {
        if (!confirm(`确定删除模型 "${modelName}" 的定价吗？`)) return;
        try {
            await API.del(`/pricing/${encodeURIComponent(modelName)}`);
            await this.load();
            showToast('定价已删除');
        } catch (e) {
            showToast('删除失败: ' + (e.message || '未知错误'));
        }
    }

    showBatchForm() {
        document.getElementById('batch-form-modal').style.display = 'flex';
        document.getElementById('batch-json').value = JSON.stringify([
            { model: 'gpt-4o', input_price: 0.18, output_price: 0.54, vendor: 'openai' },
        ], null, 2);
    }

    hideBatchForm() {
        document.getElementById('batch-form-modal').style.display = 'none';
    }

    async submitBatch() {
        const jsonStr = document.getElementById('batch-json').value.trim();
        let items;
        try {
            items = JSON.parse(jsonStr);
            if (!Array.isArray(items)) throw new Error('需要数组格式');
        } catch (e) {
            showToast('JSON 格式错误: ' + e.message);
            return;
        }

        try {
            const result = await API.post('/pricing/batch', { items });
            this.hideBatchForm();
            await this.load();
            showToast(result.message || '批量更新完成');
        } catch (e) {
            showToast('批量更新失败: ' + (e.message || '未知错误'));
        }
    }

    async reloadCache() {
        try {
            const result = await API.post('/pricing/reload');
            showToast(result.message || '刷新请求已发送');
        } catch (e) {
            showToast('刷新失败: ' + (e.message || '未知错误'));
        }
    }

    escHtml(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
}

const pricingPage = new PricingPage();
