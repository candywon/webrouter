/* API请求封装 */
const API = {
  base: '/api',

  async get(path) {
    const resp = await fetch(this.base + path);
    if (!resp.ok) throw new Error(`API Error: ${resp.status}`);
    return resp.json();
  },

  async post(path, data) {
    const resp = await fetch(this.base + path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!resp.ok) throw new Error(`API Error: ${resp.status}`);
    return resp.json();
  },

  async put(path, data) {
    const resp = await fetch(this.base + path, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!resp.ok) throw new Error(`API Error: ${resp.status}`);
    return resp.json();
  },

  async del(path) {
    const resp = await fetch(this.base + path, { method: 'DELETE' });
    if (!resp.ok) throw new Error(`API Error: ${resp.status}`);
    return resp.json();
  },
};
