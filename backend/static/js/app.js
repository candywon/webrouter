/* 应用入口 */
(function () {
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
  Router.init();
})();
