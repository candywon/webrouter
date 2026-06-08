---
layout: home

hero:
  name: "WebRouter"
  text: "统一 AI API 网关"
  tagline: 一个入口管理所有大模型服务。智能路由、成本追踪、健康监控、团队管理。
  image:
    src: /logo.svg
    alt: WebRouter
  actions:
    - theme: brand
      text: 快速开始
      link: /zh/guide/quick-start
    - theme: alt
      text: 在线演示
      link: https://webrouter-demo.fly.dev

features:
  - title: 智能路由
    details: "设置 model: auto，WebRouter 根据请求复杂度自动选择最优模型——简单请求走经济型，复杂推理走高端模型。"
    icon: 🧠
  - title: 健康监控
    details: "自动健康检测 + 延迟追踪。失效 Provider 进入冷却，流量自动切换——无需人工干预。"
    icon: 💓
  - title: 成本追踪
    details: "按模型、按 Token、按团队的实时成本核算，额度管理 + 预算告警。"
    icon: 💰
  - title: 隐私脱敏
    details: "内置脱敏引擎，请求到达上游前自动剥离手机号、身份证号、邮箱等 PII 信息。"
    icon: 🔐
  - title: 团队管理
    details: "邀请成员、分配额度、限制模型访问。每人一把独立 Key，权限隔离。"
    icon: 👥
  - title: 多数据源类型
    details: "direct、aggregate、newapi、litellm、custom——任何 OpenAI 兼容网关均可接入。一把钥匙开所有门。"
    icon: 📡
---

## 产品截图

<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(400px, 1fr)); gap: 1rem; margin-top: 2rem;">
  <img src="/screenshots/webrouter-1.png" alt="仪表盘" style="border: 1px solid var(--vp-c-divider); border-radius: 8px;" />
  <img src="/screenshots/webrouter-2.png" alt="数据源管理" style="border: 1px solid var(--vp-c-divider); border-radius: 8px;" />
  <img src="/screenshots/webrouter-3.png" alt="健康监控" style="border: 1px solid var(--vp-c-divider); border-radius: 8px;" />
  <img src="/screenshots/webrouter-4.png" alt="成本分析" style="border: 1px solid var(--vp-c-divider); border-radius: 8px;" />
  <img src="/screenshots/webrouter-5.png" alt="系统设置" style="border: 1px solid var(--vp-c-divider); border-radius: 8px;" />
</div>