// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/* WebRouter 国际化模块 — 轻量 i18n，无外部依赖 */

const I18n = {
  lang: localStorage.getItem('wr-lang') || 'zh-CN',
  data: {},
  fallbackData: {},

  async init() {
    const resp = await fetch(`/static/i18n/${this.lang}.json?t=${Date.now()}`);
    if (!resp.ok) {
      this.lang = 'zh-CN';
      const fallback = await fetch(`/static/i18n/zh-CN.json?t=${Date.now()}`);
      this.data = await fallback.json();
      return;
    }
    this.data = await resp.json();
  },

  t(key, params) {
    let text = this.data[key];
    if (text === undefined || text === null) {
      text = key;
    }
    if (params) {
      for (const [k, v] of Object.entries(params)) {
        text = text.replace(new RegExp(`\\{${k}\\}`, 'g'), String(v));
      }
    }
    return text;
  },

  setLang(lang) {
    this.lang = lang;
    localStorage.setItem('wr-lang', lang);
    location.reload();
  },

  /** 应用 data-i18n 属性到 DOM */
  applyToDOM(root) {
    root = root || document;
    root.querySelectorAll('[data-i18n]').forEach(el => {
      const key = el.dataset.i18n;
      const text = this.t(key);
      if (text === key) return;
      if (el.tagName === 'INPUT' && (el.type === 'text' || el.type === 'search')) {
        el.placeholder = text;
      } else if (el.tagName === 'INPUT' && el.type === 'submit') {
        el.value = text;
      } else if (el.children.length > 0) {
        // 元素有子元素 — 只替换最后一个文本节点，保留子元素
        let lastText = null;
        for (let i = el.childNodes.length - 1; i >= 0; i--) {
          if (el.childNodes[i].nodeType === Node.TEXT_NODE && el.childNodes[i].textContent.trim()) {
            lastText = el.childNodes[i];
            break;
          }
        }
        if (lastText) {
          lastText.textContent = lastText.textContent.replace(/\S.*$/s, '') + text;
        } else {
          el.appendChild(document.createTextNode(text));
        }
      } else {
        el.textContent = text;
      }
    });
    // data-i18n-title
    root.querySelectorAll('[data-i18n-title]').forEach(el => {
      const text = this.t(el.dataset.i18nTitle);
      if (text !== el.dataset.i18nTitle) el.title = text;
    });
  }
};

// 暴露到全局
window.I18n = I18n;
