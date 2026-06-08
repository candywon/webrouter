---
layout: home

hero:
  name: "WebRouter"
  text: "Unified AI API Gateway"
  tagline: One API key to manage all your LLM providers. Smart routing, cost tracking, health monitoring, and team management.
  image:
    src: /logo.svg
    alt: WebRouter
  actions:
    - theme: brand
      text: Get Started
      link: /guide/quick-start
    - theme: alt
      text: Live Demo
      link: https://demo.webrouter.tech

features:
  - title: Smart Routing
    details: "Set model: auto and WebRouter picks the optimal model based on request complexity. Simple queries get fast/cheap models, complex reasoning gets powerful ones."
    icon: 🧠
  - title: Health Monitoring
    details: "Automatic health checks with latency tracking. Dead providers enter cooldown; traffic shifts to healthy alternatives - no manual intervention needed."
    icon: 💓
  - title: Cost Tracking
    details: "Real-time cost accounting per model, per token, per team. Billing reports, quota management, and budget alerts built in."
    icon: 💰
  - title: Privacy & Desensitization
    details: "Built-in desensitization engine strips PII (phone numbers, ID cards, emails) from requests before they reach upstream providers."
    icon: 🔐
  - title: Team Management
    details: "Invite team members, assign quotas, restrict model access. Each member gets a unique API key with scoped permissions."
    icon: 👥
  - title: Multi-Provider Support
    details: "direct, aggregate, newapi, litellm, custom - any OpenAI-compatible provider. One gateway, all your LLMs."
    icon: 📡
---

## Screenshots

<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(400px, 1fr)); gap: 1rem; margin-top: 2rem;">
  <img src="/screenshots/webrouter-1.png" alt="Dashboard" style="border: 1px solid var(--vp-c-divider); border-radius: 8px;" />
  <img src="/screenshots/webrouter-2.png" alt="Provider Management" style="border: 1px solid var(--vp-c-divider); border-radius: 8px;" />
  <img src="/screenshots/webrouter-3.png" alt="Health Monitoring" style="border: 1px solid var(--vp-c-divider); border-radius: 8px;" />
  <img src="/screenshots/webrouter-4.png" alt="Billing Analytics" style="border: 1px solid var(--vp-c-divider); border-radius: 8px;" />
  <img src="/screenshots/webrouter-5.png" alt="System Settings" style="border: 1px solid var(--vp-c-divider); border-radius: 8px;" />
</div>