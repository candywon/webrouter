/* 团队管理页面逻辑 */
const TeamPage = {
  async load() {
    try {
      const data = await API.get('/team/members');
      const el = document.getElementById('team-content');
      if (!el) return;
      if (!data.members || data.members.length === 0) {
        el.innerHTML = '<div class="empty-state"><div class="icon">👥</div><p>暂无团队成员<br>点击"邀请成员"添加</p></div>';
        return;
      }
      el.innerHTML = `<table>
        <thead><tr><th>用户名</th><th>角色</th><th>总额度</th><th>已用</th><th>剩余</th><th>操作</th></tr></thead>
        <tbody>${data.members.map(m => `
          <tr>
            <td>${m.display_name || m.username || '-'}</td>
            <td><span class="badge badge-info">${m.role || '-'}</span></td>
            <td>${m.quota ? formatNumber(m.quota.quota_total) : '-'}</td>
            <td>${m.quota ? formatNumber(m.quota.quota_used) : '-'}</td>
            <td>${m.quota ? formatNumber(m.quota.quota_remaining) : '-'}</td>
            <td>
              <button class="btn" onclick="TeamPage.editQuota(${m.id})">编辑额度</button>
              <button class="btn" onclick="TeamPage.removeMember(${m.id})">移除</button>
            </td>
          </tr>
        `).join('')}</tbody>
      </table>`;
    } catch (e) {
      console.error('Failed to load team:', e);
    }
  },

  async editQuota(userId) {
    const quota = prompt('设定总额度 (tokens):', '1000000');
    if (!quota) return;
    try {
      await API.put(`/team/members/${userId}`, { quota_total: parseInt(quota) });
      showToast('额度已更新');
      this.load();
    } catch (e) { showToast('更新失败'); }
  },

  async removeMember(userId) {
    if (!confirm('确定移除此成员？')) return;
    try {
      await API.del(`/team/members/${userId}`);
      showToast('已移除');
      this.load();
    } catch (e) { showToast('移除失败'); }
  },
};
