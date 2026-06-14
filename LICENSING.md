# WebRouter — 许可证与版本分界方案

> 本文档定义 WebRouter 的开源/商业双轨策略，作为后续法律文件（LICENSE、CLA、EULA）与产品路线图的基线。

---

## 一、版本矩阵

| 版本 | 代号 | 协议 | 目标用户 | 交付形式 |
|------|------|------|----------|----------|
| **Community Edition (CE)** | `webrouter-ce` | BSL 1.1 → Apache-2.0 | 个人开发者、小团队、技术爱好者 | 源码 + 二进制 |
| **Enterprise Edition (EE)** | `webrouter-ee` | 商业许可 (Proprietary EULA) | 中大型企业、金融/政企/合规场景 | 二进制 + 许可证 Key |
| **Cloud (SaaS)** | `webrouter.cloud` | 服务订阅 (ToS) | 不想运维的所有客户 | 托管服务 |

> 三档定价层共享同一内核架构；EE 与 Cloud 通过**插件模块 + 许可证校验中间件**叠加在 CE 之上。

---

## 二、协议选择与依据

### 2.1 CE 选 BSL 1.1（过渡期后自动转 Apache-2.0）

**核心思路**：前期用 BSL 1.1 防止二次包装倒卖，Change Date 后自动转纯 Apache-2.0，社区与商业兼顾。

| 维度 | 决策 |
|------|------|
| **吸引社区** | 自用免费、可修改、可部署，与 Apache-2.0 体验一致 |
| **防倒卖** | BSL 条款禁止将本作品或衍生作品作为商业产品/服务出售 |
| **防白嫖** | 云厂商无法拿去做托管服务售卖 |
| **时效性** | Change Date 后自动转 Apache-2.0，社区不会觉得"反开源" |
| **专利保护** | 转 Apache-2.0 后含明示专利授权 + 反诉自动终止条款 |

**BSL 1.1 关键条款**：

```
Business Source License 1.1

Parameters:
  Licensor:        Jianlin Huang
  Licensed Work:   WebRouter Community Edition
                 (webrouter-ce, 包括 backend/ 和 wr-proxy/)
  Additional Use Grant:
    You may use the Licensed Work for non-commercial purposes,
    including production internal use, provided that you do not
    offer the Licensed Work or any derivative thereof as a
    commercial product or service (e.g., managed service,
    resold SaaS, OEM embedding).
  Change Date:     2029-06-01
  Change License:  Apache-2.0
```

**条款解读**：
- ✅ 允许：个人使用、企业内部生产部署、学习研究、修改代码、提交 PR
- ❌ 禁止：将 WebRouter（含衍生品）包装成商业产品售卖、提供付费托管服务、OEM 嵌入闭源产品转售
- 🔄 Change Date（2029-06-01）后：自动转为 Apache-2.0，所有商业限制消失

**业界先例**：

| 项目 | BSL Change Date | 状态 |
|------|-----------------|------|
| HashiCorp Terraform/Vault | 4 年 | 已转 BSL，后被 IBM 收购 |
| CockroachDB | 3 年 | 稳定运行 |
| Sentry | 3 年 | 稳定运行 |
| MariaDB MaxScale | 5 年 | 稳定运行 |
| **WebRouter CE** | **3 年（2029-06-01）** | 首次协议落地 |

**为什么不用纯 Apache-2.0**：
- 开源后最现实的风险不是云厂商白嫖，而是**开发者二次包装后以更低价格卖给企业客户**——纯 Apache 无法阻止
- BSL 1.1 是目前最平衡的方案：自用免费 + 防商业转售 + 有期限 → 社区可接受

**为什么不用 AGPL-3.0**：
- AGPL 的"传染性"会劝退想集成 WebRouter 的企业（SaaS 场景下必须公开全部源码）
- BSL 只限制"商业售卖"，不限制"使用"，对企业更友好
- AGPL 无法设定过期时间，一旦 AGPL 永远 AGPL

### 2.2 必须配套：贡献者许可协议 (CLA)

**所有外部贡献者**提交 PR 前必须签署 CLA，授予项目所有人：

1. 永久、全球、免费的版权许可
2. 永久、全球、免费的专利许可
3. **再许可权（sublicense right）**——这是未来调整协议条款（BSL → Apache → 商业）的法律基础

> 不签 CLA = 未来想改协议时被一行代码卡住（HashiCorp 改 BSL 时就靠 CLA 才走通）。

### 2.3 EE 用专有 EULA

要点：
- 按席位 / 按节点 / 按调用量计费
- 禁止反编译、二次分发、白牌出售
- 包含 SLA 条款、责任上限、数据处理协议（DPA）
- 中国客户加挂《软件正版化使用承诺》

---

## 三、功能分界（CE vs EE vs Cloud）

> 原则：CE 覆盖"单机能跑、个人能用"；EE 解决"企业能合规、能高可用"；Cloud 解决"完全不想运维"。

### 3.1 核心网关（基于 wr-proxy + backend）

| 能力 | CE | EE | Cloud |
|------|:--:|:--:|:--:|
| 多 Provider 接入（direct/aggregate/newapi/oneapi/litellm/custom） | ✅ | ✅ | ✅ |
| 智能路由（auto/smart 模型选择） | ✅ | ✅ | ✅ |
| 健康检查与冷却 | ✅ | ✅ | ✅ |
| 请求脱敏（desensitize） | 基础规则 | + 自定义规则引擎 + 正则库订阅 | ✅ |
| 重试与降级 | ✅ | + 跨区域降级、跨账号降级 | ✅ |
| 流式响应 | ✅ | ✅ | ✅ |
| 上游 API Key 中心化代付 | ❌ | ❌ | ✅（关键卖点）|

### 3.2 数据存储与扩展性

| 能力 | CE | EE | Cloud |
|------|:--:|:--:|:--:|
| SQLite 单机 | ✅ | ✅ | — |
| MySQL / PostgreSQL | ✅（基础） | ✅（连接池、读写分离） | ✅ |
| Redis 缓存 | ✅ | ✅ | ✅ |
| 集群部署（多实例 + 共享状态） | ❌ | ✅ | ✅ |
| 跨区域多活 / 灾备 | ❌ | ✅ | ✅ |
| 数据加密（at-rest, AES-256） | ❌ | ✅ | ✅ |
| KMS / HSM 集成（AWS KMS、阿里 KMS） | ❌ | ✅ | ✅ |

### 3.3 用户、权限与组织

| 能力 | CE | EE | Cloud |
|------|:--:|:--:|:--:|
| 单管理员账号 | ✅ | ✅ | ✅ |
| 多用户 + 基础角色（admin/viewer） | ✅ | ✅ | ✅ |
| 组织树 / 多部门隔离 | 单组织 | ✅ 多组织无限层级 | ✅ |
| RBAC 细粒度权限（资源级） | ❌ | ✅ | ✅ |
| SSO（SAML 2.0 / OIDC / OAuth2） | ❌ | ✅ | ✅ |
| LDAP / Active Directory 同步 | ❌ | ✅ | ✅ |
| 双因素认证 (2FA / TOTP) | 基础 | + 强制策略、WebAuthn | ✅ |
| SCIM 用户自动同步 | ❌ | ✅ | ✅ |

### 3.4 监控、告警与合规

| 能力 | CE | EE | Cloud |
|------|:--:|:--:|:--:|
| 健康监控 + 历史曲线 | 7 天 | 无限保留 | 无限保留 |
| 告警规则（基础阈值） | ✅ | ✅ | ✅ |
| 告警通道（邮件） | ✅ | ✅ | ✅ |
| 告警通道（钉钉/飞书/企微/Slack/PagerDuty/Webhook） | ❌ | ✅ | ✅ |
| 告警分级路由 + 值班排班 | ❌ | ✅ | ✅ |
| 审计日志（操作 / 登录 / 配置变更） | ❌ | ✅ 全量、不可篡改 | ✅ |
| 合规报表（SOC2 / ISO 27001 / 等保） | ❌ | ✅ 一键导出 | ✅ |
| 数据出境合规（区域路由策略） | ❌ | ✅ | ✅ |

### 3.5 成本与计费

| 能力 | CE | EE | Cloud |
|------|:--:|:--:|:--:|
| 用量统计 + 成本核算 | ✅ | ✅ | ✅ |
| 模型定价管理 | ✅ | ✅ | ✅ |
| 配额管理（按 Token） | ✅ | ✅ | ✅ |
| 按部门 / 项目 / 业务线分摊 | ❌ | ✅ | ✅ |
| 预算预警 + 自动停用 | 基础 | + 多级阈值、智能预测 | ✅ |
| 财务报表导出（PDF/Excel/API） | CSV 导出 | ✅ 全格式 + 定时邮件 | ✅ |
| 发票与对账（企业财务对接） | ❌ | ✅ | ✅ |

### 3.6 高级路由策略

| 能力 | CE | EE | Cloud |
|------|:--:|:--:|:--:|
| 模型别名 (modelaliases) | ✅ | ✅ | ✅ |
| 模型分级 (modelgrades) | ✅ | ✅ | ✅ |
| 按部门配额隔离 | ❌ | ✅ | ✅ |
| 按业务标签智能路由 | ❌ | ✅ | ✅ |
| 成本最优自动路由（基于实时单价） | ❌ | ✅ | ✅ |
| A/B 测试与灰度路由 | ❌ | ✅ | ✅ |
| 自定义路由规则 DSL | ❌ | ✅ | ✅ |

### 3.7 知识库与会话

| 能力 | CE | EE | Cloud |
|------|:--:|:--:|:--:|
| 知识库管理（knowledge） | ✅ 单租户 | ✅ 多租户、向量检索 | ✅ |
| 会话历史 (session) | 本地保留 | + 加密归档、按合规导出 | ✅ |
| RAG 增强路由 | 基础 | + 混合检索、重排序 | ✅ |

### 3.8 运维与生命周期

| 能力 | CE | EE | Cloud |
|------|:--:|:--:|:--:|
| 一键安装脚本 | ✅ | ✅ | — |
| Docker Compose 部署 | ✅ | ✅ | — |
| Kubernetes Helm Chart | 社区版 | ✅ 官方维护 + Operator | — |
| 在线热升级（零停机） | ❌ | ✅ | ✅ |
| 自动备份与一键恢复 | 手动 | ✅ 定时 + S3/OSS | ✅ |
| 多环境管理（dev/staging/prod 隔离） | ❌ | ✅ | ✅ |

### 3.9 支持与服务

| 维度 | CE | EE | Cloud |
|------|----|----|------|
| 文档 | 开源文档 | + 私有最佳实践库 | + 私有最佳实践库 |
| 技术支持 | GitHub Issues / 社区 | 工单 + 邮件，工作时间 4h 响应 | 工单 24×7 |
| SLA | 无 | 99.5% | 99.9% |
| 专属客户成功经理 | ❌ | 年付客户 ✅ | 企业套餐 ✅ |
| 上门部署 / 培训 | ❌ | 可选 | — |
| 定制开发 | ❌ | 单独报价 | — |

---

## 四、技术实现要点（保证 EE 闭源功能不外泄）

### 4.1 模块化架构

```
webrouter-ce (BSL 1.1 → Apache-2.0, 公开仓库)
  └── 提供插件接口 (Plugin SDK)
       ├── plugins/sso/         (闭源, EE-only)
       ├── plugins/audit/       (闭源, EE-only)
       ├── plugins/cluster/     (闭源, EE-only)
       └── plugins/billing-pro/ (闭源, EE-only)
```

- CE 仓库定义清晰的 **Plugin 接口（hook + interface）**
- EE 模块以**二进制 .so / Go plugin / Python wheel** 形式独立分发
- EE 启动时校验许可证 Key（公钥签名 JWT，含到期时间、节点数上限）

### 4.2 仓库布局

| 仓库 | 可见性 | 协议 |
|------|--------|------|
| `github.com/candywon/webrouter` | 公开 | BSL 1.1 → Apache-2.0 |
| `github.com/candywon/webrouter-docs` | 公开 | CC-BY-4.0 |
| `github.com/candywon/webrouter-helm` | 公开 | Apache-2.0 |
| `github.com/candywon/webrouter-ee`（私有）| 内部 | 专有 |
| `github.com/candywon/webrouter-cloud`（私有）| 内部 | 专有 |

### 4.3 防绕过措施

- **不在 CE 里留 EE 功能的禁用开关**——开关本身就是入侵点
- **EE 二进制做混淆 + 完整性校验**
- 许可证文件需联网每 30 天回 home，离线超期降级到只读模式
- **法律层面**：EULA 明确反向工程禁止条款

---

## 五、社区治理与协议演进权

### 5.1 必须建立的法律基础设施

| 文件 | 作用 | 优先级 |
|------|------|--------|
| `LICENSE`（BSL 1.1 全文，含 Change Date → Apache-2.0 条款） | 协议本体 | 🔴 立刻 |
| `NOTICE` | 第三方依赖归属声明 | 🔴 立刻 |
| `CONTRIBUTING.md` | 贡献规则 + 引导签 CLA | 🔴 立刻 |
| `CLA.md` 或 EasyCLA 自动化 | 贡献者权利转移 | 🔴 立刻 |
| 源文件版权头模板 | SPDX-License-Identifier | 🟡 1 月内 |
| `SECURITY.md` | 漏洞披露流程 | 🟡 1 月内 |
| `CODE_OF_CONDUCT.md` | 社区行为规范 | 🟢 3 月内 |
| `GOVERNANCE.md` | 项目治理结构 | 🟢 6 月内 |

### 5.2 版权持有人建议

- 如果是**个人项目**：写个人名（中英对照）
- 如果是**公司主导**：注册一个**主体公司**，所有版权统一归该公司
- **强烈避免**：混合署名（一会儿个人一会儿公司），未来融资 / 出售时会被律师挑出来重新签字

---

## 六、定价方向（仅供决策参考）

| 套餐 | 定价模型 | 参考价位（年付） |
|------|---------|------------------|
| CE | 免费 | 0 |
| EE - 团队版 | 5–20 节点，基础企业功能 | $5K–$15K |
| EE - 企业版 | 不限节点，全部 EE 功能，SLA 99.5% | $30K–$100K+ |
| EE - 旗舰版 | 全部 + 定制开发 + 上门支持 | 单独报价 |
| Cloud Starter | 按调用量 ($0.5 / 百万 token routing fee) | 按量 |
| Cloud Business | 含 SSO/审计/SLA 99.9% | $99–$999 / 月 |
| Cloud Dedicated | 单租户独立集群 | $5K+/月 |

---

## 七、Roadmap：从 Apache 到稳定双轨

| 阶段 | 时间窗 | 关键动作 |
|------|--------|----------|
| **Phase 0 — 立法** | 即刻 | 落地 LICENSE (BSL 1.1) / CLA / 版权头 / EULA 模板 |
| **Phase 1 — 攒社区** | 0–6 月 | BSL 1.1 开源（自用免费），文档 + 案例；同步准备 EE 插件接口 |
| **Phase 2 — 商业化预演** | 6–12 月 | 内部研发 EE 插件接口与首批闭源模块（SSO + 审计） |
| **Phase 3 — EE 发布** | 12 月起 | 推出 EE 1.0；CE 仍为 BSL，防倒卖 |
| **Phase 4 — Cloud 上线** | 12–18 月 | 推出托管版，主打"零运维 + 上游代付" |
| **Phase 5 — 协议演进** | 2029-06-01 | Change Date 到期，CE 自动转 Apache-2.0；如届时仍需防御，可续签新版 BSL 或切 AGPL |

---

## 八、最重要的三件事

1. **第一天就让贡献者签 CLA**——这是未来调整协议条款的法律前提
2. **从一开始就在 README 中划清"永远开源" vs "属于商业版"**——避免后续被指责"反开源"
3. **不要把 EE 功能塞进 CE 仓库做 if-enterprise 判断**——架构上彻底分离，否则法律和工程都是泥潭
4. **BSL 的 Change Date 写死在 LICENSE 里**——不可单方面推迟或修改，这是社区信任的基石
