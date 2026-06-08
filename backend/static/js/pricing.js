// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

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
            html = `<div class="empty-state"><p>${I18n.t('pricing.noPricing')}</p><p class="hint">${I18n.t('pricing.addPricingHint')}</p></div>`;
        } else {
            html += `<div class="table-wrap"><table>
                <thead><tr>
                    <th>${I18n.t('common.model')}</th><th>${I18n.t('pricing.vendor')}</th><th>${I18n.t('pricing.inputPrice')}</th><th>${I18n.t('pricing.outputPrice')}</th><th>${I18n.t('common.default')}</th><th>${I18n.t('common.notes')}</th><th>${I18n.t('common.actions')}</th>
                </tr></thead><tbody>`;

            for (const p of this.pricing) {
                const vendorCls = vendorColor[p.vendor] || 'vendor-other';
                html += `
                <tr>
                    <td><code class="model-name">${this.escHtml(p.model)}</code></td>
                    <td><span class="badge ${vendorCls}">${this.escHtml(p.vendor || 'other')}</span></td>
                    <td>${this.priceToYuan(p.input_price)}/M</td>
                    <td>${this.priceToYuan(p.output_price)}/M</td>
                    <td>${p.is_default ? '<span class="badge badge-healthy">' + I18n.t('common.default') + '</span>' : ''}</td>
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

        let opts = `<option value="">${I18n.t('pricing.allVendors')}</option>`;
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
        document.getElementById('pricing-form-title').textContent = I18n.t("pricing.addFormTitle");
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
        document.getElementById('pricing-form-title').textContent = I18n.t("pricing.editFormTitle");
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

        if (!data.model) { showToast(I18n.t("pricing.modelNameRequired")); return; }

        try {
            if (this.editingModel) {
                await API.put(`/pricing/${encodeURIComponent(this.editingModel)}`, data);
            } else {
                await API.post('/pricing/', data);
            }
            this.hideForm();
            await this.load();
            showToast(I18n.t("pricing.saveSuccess"));
        } catch (e) {
            showToast(I18n.t("common.saveFailed") + (e.message || I18n.t("common.unknownError")));
        }
    }

    async deletePricing(modelName) {
        if (!confirm(I18n.t('pricing.confirmDelete', {modelName}))) return;
        try {
            await API.del(`/pricing/${encodeURIComponent(modelName)}`);
            await this.load();
            showToast(I18n.t("pricing.deleted"));
        } catch (e) {
            showToast(I18n.t("common.deleteFailed") + (e.message || I18n.t("common.unknownError")));
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
            if (!Array.isArray(items)) throw new Error(I18n.t("common.arrayRequired"));
        } catch (e) {
            showToast(I18n.t("common.jsonError") + e.message);
            return;
        }

        try {
            const result = await API.post('/pricing/batch', { items });
            this.hideBatchForm();
            await this.load();
            showToast(result.message || I18n.t("pricing.batchDone"));
        } catch (e) {
            showToast(I18n.t("pricing.batchFailed") + (e.message || I18n.t("common.unknownError")));
        }
    }

    async reloadCache() {
        try {
            const result = await API.post('/pricing/reload');
            showToast(result.message || I18n.t("pricing.reloadSent"));
        } catch (e) {
            showToast(I18n.t("pricing.reloadFailed") + (e.message || I18n.t("common.unknownError")));
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
