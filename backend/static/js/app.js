// SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
// SPDX-License-Identifier: BUSL-1.1

/* 应用入口 */
(async function () {
  // Init i18n before anything renders
  await I18n.init();
  I18n.applyToDOM();

  Router.register('/login', LoginPage);
  Router.register('/', DashboardPage);
  Router.register('/providers', providersPage);
  Router.register('/channels', channelPage);
  Router.register('/provider-channels', channelPage);
  Router.register('/tokens', tokensPage);
  Router.register('/pricing', pricingPage);
  Router.register('/modelgrades', modelGradesPage);
  Router.register('/desensitize', desensitizePage);
  Router.register('/monitor', MonitorPage);
  Router.register('/alerts', AlertPage);
  Router.register('/billing', BillingPage);
  Router.register('/team', TeamPage);
  Router.register('/cli', CLIPage);
  Router.register('/reqcache', reqCachePage);
  Router.register('/api-test', ApiTestPage);
  Router.register('/knowledge', KnowledgePage);
  Router.register('/settings', settingsPage);

  const path = window.location.hash.slice(1) || '/';
  if (path !== '/login') {
    try {
      const res = await fetch('/api/auth/status', { credentials: 'same-origin' });
      const data = await res.json();
      if (!data.authenticated) window.location.hash = '/login';
    } catch (_) {
      window.location.hash = '/login';
    }
  }

  Router.init();
})();
