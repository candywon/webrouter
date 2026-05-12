/* 计费统计页面逻辑 */
const BillingPage = {
  async load() {
    try {
      const data = await API.get('/billing/cost?days=30');
      const el = document.getElementById('billing-content');
      if (!el) return;
      if (!data.data || data.data.length === 0) {
        el.innerHTML = '<div class="empty-state"><div class="icon">💰</div><p>暂无计费数据</p></div>';
        return;
      }
      let totalCents = 0;
      data.data.forEach(d => totalCents += (d.cost_cents || 0));
      el.innerHTML = `
        <div class="stats-grid" style="margin-bottom:20px">
          <div class="stat-card">
            <div class="stat-value" style="color:var(--accent)">${formatYuan(totalCents)}</div>
            <div class="stat-label">30天总成本</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">${data.data.length}</div>
            <div class="stat-label">使用模型数</div>
          </div>
        </div>
        <table>
          <thead><tr><th>模型</th><th>输入Token</th><th>输出Token</th><th>成本</th></tr></thead>
          <tbody>${data.data.map(d => `
            <tr>
              <td>${d.model_name}</td>
              <td>${formatNumber(d.input_tokens)}</td>
              <td>${formatNumber(d.output_tokens)}</td>
              <td>${formatYuan(d.cost_cents)}</td>
            </tr>
          `).join('')}</tbody>
        </table>`;
    } catch (e) {
      console.error('Failed to load billing:', e);
    }
  },
};
