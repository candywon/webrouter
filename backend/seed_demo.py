# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""Demo 模式种子数据 — 确定性生成，random.seed(42)"""

import json
import random
from datetime import datetime, timedelta

random.seed(42)

# 6 个 Demo Provider
_DEMO_PROVIDERS = [
    {'name': 'OpenAI', 'type': 'direct', 'base_url': 'https://api.openai.com/v1',
     'api_key': 'sk-demo-openai-placeholder-key',
     'models': json.dumps(['gpt-4o', 'gpt-4o-mini']), 'tags': json.dumps(['direct', 'openai']),
     'weight': 100, 'priority': 90, 'enabled': True, 'status': 'healthy',
     'notes': 'Demo: OpenAI'},
    {'name': 'Anthropic', 'type': 'direct', 'base_url': 'https://api.anthropic.com',
     'api_key': 'sk-demo-ant-placeholder-key',
     'models': json.dumps(['claude-sonnet-4', 'claude-haiku-4-5']), 'tags': json.dumps(['direct', 'anthropic']),
     'weight': 100, 'priority': 80, 'enabled': True, 'status': 'healthy',
     'notes': 'Demo: Anthropic'},
    {'name': 'DeepSeek', 'type': 'direct', 'base_url': 'https://api.deepseek.com/v1',
     'api_key': 'sk-demo-deepseek-placeholder-key',
     'models': json.dumps(['deepseek-chat', 'deepseek-reasoner']), 'tags': json.dumps(['direct', 'deepseek']),
     'weight': 100, 'priority': 70, 'enabled': True, 'status': 'healthy',
     'notes': 'Demo: DeepSeek'},
    {'name': 'DashScope/Qwen', 'type': 'direct', 'base_url': 'https://dashscope.aliyuncs.com/compatible-mode/v1',
     'api_key': 'sk-demo-dashscope-placeholder-key',
     'models': json.dumps(['qwen-plus', 'qwen-turbo']), 'tags': json.dumps(['direct', 'qwen']),
     'weight': 100, 'priority': 60, 'enabled': True, 'status': 'healthy',
     'notes': 'Demo: DashScope'},
    {'name': 'OpenRouter', 'type': 'direct', 'base_url': 'https://openrouter.ai/api/v1',
     'api_key': 'sk-demo-openrouter-placeholder-key',
     'models': json.dumps(['openai/gpt-4o', 'anthropic/claude-sonnet-4']), 'tags': json.dumps(['aggregate', 'openrouter']),
     'weight': 80, 'priority': 50, 'enabled': True, 'status': 'warning',
     'notes': 'Demo: OpenRouter'},
    {'name': 'Custom Gateway', 'type': 'custom', 'base_url': 'http://localhost:3000/v1',
     'api_key': 'sk-demo-custom-placeholder-key',
     'models': json.dumps(['custom-model-v1', 'custom-model-v2']), 'tags': json.dumps(['custom']),
     'weight': 50, 'priority': 10, 'enabled': True, 'status': 'dead',
     'notes': 'Demo: 未配置的网关示例'},
]

# 5 个组织（树形）
_DEMO_ORGS = [
    {'name': 'Engineering', 'org_type': 'company', 'parent_id': None},
    {'name': 'Backend', 'org_type': 'department', 'parent_id': 0},  # idx 0 会被替换
    {'name': 'Frontend', 'org_type': 'department', 'parent_id': 0},
    {'name': 'ML Research', 'org_type': 'company', 'parent_id': None},
    {'name': 'LLM Ops', 'org_type': 'department', 'parent_id': 3},
    {'name': 'Product', 'org_type': 'company', 'parent_id': None},
]

# 6 Token（关联到不同组织）
_DEMO_TOKENS = [
    {'name': '张三-后端', 'member_email': 'zhangsan@demo.com', 'org_idx': 1,
     'models': json.dumps(['gpt-4o', 'deepseek-chat']), 'quota_total': 500000,
     'rate_limit_rpm': 100, 'smart_downgrade': True, 'desensitize_enabled': True},
    {'name': '李四-前端', 'member_email': 'lisi@demo.com', 'org_idx': 2,
     'models': json.dumps(['gpt-4o-mini', 'qwen-plus']), 'quota_total': 200000,
     'rate_limit_rpm': 60, 'smart_downgrade': True, 'desensitize_enabled': False},
    {'name': '王五-LLM', 'member_email': 'wangwu@demo.com', 'org_idx': 4,
     'models': json.dumps(['claude-sonnet-4', 'deepseek-reasoner']), 'quota_total': 1000000,
     'rate_limit_rpm': 200, 'smart_downgrade': False, 'desensitize_enabled': True},
    {'name': '赵六-产品', 'member_email': 'zhaoliu@demo.com', 'org_idx': 5,
     'models': json.dumps(['gpt-4o', 'qwen-turbo']), 'quota_total': 300000,
     'rate_limit_rpm': 80, 'smart_downgrade': True, 'desensitize_enabled': False},
    {'name': '孙七-后端', 'member_email': 'sunqi@demo.com', 'org_idx': 1,
     'models': json.dumps(['deepseek-chat', 'gpt-4o-mini']), 'quota_total': 150000,
     'rate_limit_rpm': 50, 'smart_downgrade': True, 'desensitize_enabled': True},
    {'name': '周八-ML', 'member_email': 'zhouba@demo.com', 'org_idx': 3,
     'models': json.dumps(['claude-sonnet-4', 'gpt-4o']), 'quota_total': 800000,
     'rate_limit_rpm': 150, 'smart_downgrade': False, 'desensitize_enabled': True},
]

# 3 个告警规则
_DEMO_ALERT_RULES = [
    {'name': '高错误率告警', 'condition_type': 'error_rate', 'condition_config': '{"threshold":0.1,"window_minutes":5}',
     'level': 'critical', 'channels': '["wechat"]', 'enabled': True},
    {'name': '额度预警', 'condition_type': 'quota_remaining', 'condition_config': '{"threshold":0.2,"scope":"token"}',
     'level': 'warning', 'channels': '["email"]', 'enabled': True},
    {'name': 'Provider 断开', 'condition_type': 'provider_down', 'condition_config': '{"max_retries":3,"window_minutes":10}',
     'level': 'critical', 'channels': '["wechat","email"]', 'enabled': True},
]

# 3 个脱敏规则
_DEMO_DESENSITIZE_RULES = [
    {'name': '姓名脱敏', 'type': 'regex', 'pattern': '(?<=[\\u4e00-\\u9fa5]).{0,1}(?=[\\u4e00-\\u9fa5])',
     'category': 'NAME', 'level': 'standard'},
    {'name': '地址脱敏', 'type': 'regex', 'pattern': '(?:省|市|区|县|镇|村|路|街|巷|号|栋|单元|室)',
     'category': 'ADDRESS', 'level': 'standard'},
    {'name': '手机号脱敏', 'type': 'regex', 'pattern': '(1[3-9]\\d)\\d{4}(\\d{4})',
     'category': 'PHONE', 'level': 'standard'},
]

# 3 个额外模型别名
_DEMO_MODEL_ALIASES = [
    {'alias': 'gpt4', 'target': 'gpt-4o'},
    {'alias': 'claude-sonnet', 'target': 'claude-sonnet-4'},
    {'alias': 'deepseek-v3', 'target': 'deepseek-chat'},
]


def seed_demo_data(app, reset=False):
    """播种 Demo 数据。reset=True 时先清空再播种。"""
    from extensions import db
    from models.wr_models import AdminUser, WRToken, RequestLog, AlertRule, AlertHistory, ChannelHealth, DesensitizeRule, ModelAlias, ProviderExt, Org
    from models.provider import Provider

    # 幂等检查：已有 Demo Token 则跳过（比 Provider 更准确，不与 seed_defaults 冲突）
    demo_token_names = [t['name'] for t in _DEMO_TOKENS]
    existing_tokens = WRToken.query.filter(
        WRToken.name.in_(demo_token_names)
    ).count()
    if not reset and existing_tokens >= 4:
        # 即使跳过完整播种，也确保 demo 管理员存在（种子数据可能不完整）
        admin = AdminUser.query.filter_by(username='demo').first()
        if not admin:
            admin = AdminUser(username='demo', enabled=True)
            admin.set_password('demo123456')
            db.session.add(admin)
            db.session.commit()
            app.logger.info('[demo] 补充创建 demo 管理员')
        app.logger.info('[demo] 种子数据已存在，跳过')
        return

    if reset:
        app.logger.warning('[demo] 重置种子数据...')
        # 清空顺序：依赖多的先删
        AlertHistory.query.delete()
        RequestLog.query.delete()
        ChannelHealth.query.delete()
        AlertRule.query.delete()
        DesensitizeRule.query.delete()
        WRToken.query.delete()
        ProviderExt.query.delete()
        Provider.query.delete()
        Org.query.delete()
        # 保留 AdminUser
        # 保留 ModelAlias（不删默认别名）
        db.session.commit()

    now = datetime.utcnow()

    # ---- 1. AdminUser ----
    admin = AdminUser.query.filter_by(username='demo').first()
    if not admin:
        admin = AdminUser(username='demo', enabled=True)
        admin.set_password('demo123456')
        db.session.add(admin)
        app.logger.info('[demo] 创建管理员 demo/demo123456')

    # ---- 2. Orgs ----
    org_map = {}
    for i, od in enumerate(_DEMO_ORGS):
        existing = Org.query.filter_by(name=od['name']).first()
        if existing:
            org_map[i] = existing
        else:
            parent_id = None
            if od['parent_id'] is not None:
                parent_id = org_map[od['parent_id']].id
            o = Org(name=od['name'], org_type=od['org_type'], parent_id=parent_id)
            db.session.add(o)
            db.session.flush()
            org_map[i] = o

    # ---- 3. Providers ----
    provider_map = {}
    for pd in _DEMO_PROVIDERS:
        existing = Provider.query.filter_by(name=pd['name']).first()
        if existing:
            provider_map[pd['name']] = existing
            continue
        p = Provider(
            name=pd['name'], type=pd['type'], base_url=pd['base_url'],
            api_key=pd.get('api_key', ''),
            models=pd['models'], tags=pd['tags'], weight=pd['weight'],
            priority=pd['priority'], enabled=pd['enabled'], status=pd['status'],
            notes=pd['notes'],
        )
        db.session.add(p)
        db.session.flush()
        pe = ProviderExt(provider_id=p.id, proxy_enabled=pd['enabled'], priority=pd['priority'], weight=pd['weight'])
        db.session.add(pe)
        provider_map[pd['name']] = p

    # ---- 4. Tokens ----
    token_map = {}
    for td in _DEMO_TOKENS:
        existing = WRToken.query.filter_by(name=td['name']).first()
        if existing:
            token_map[td['name']] = existing
            continue
        t = WRToken(
            name=td['name'], key=WRToken.generate_key(),
            org_id=org_map[td['org_idx']].id,
            member_email=td['member_email'], models=td['models'],
            quota_total=td['quota_total'], rate_limit_rpm=td['rate_limit_rpm'],
            smart_downgrade=td['smart_downgrade'],
            desensitize_enabled=td['desensitize_enabled'],
            enabled=True,
        )
        db.session.add(t)
        db.session.flush()
        token_map[td['name']] = t

    # ---- 5. Alert Rules ----
    for ad in _DEMO_ALERT_RULES:
        existing = AlertRule.query.filter_by(name=ad['name']).first()
        if not existing:
            r = AlertRule(
                name=ad['name'], condition_type=ad['condition_type'],
                condition_config=ad['condition_config'], level=ad['level'],
                channels=ad['channels'], enabled=ad['enabled'],
            )
            db.session.add(r)

    # ---- 6. Alert History ----
    if AlertHistory.query.count() == 0:
        alert_msgs = [
            ('error_rate', 'GPT-4o 错误率超过 10%（最近5分钟）', 'critical'),
            ('error_rate', 'DeepSeek 错误率超过 10%（最近5分钟）', 'critical'),
            ('quota_remaining', 'Token "张三-后端" 剩余额度不足 20%', 'warning'),
            ('quota_remaining', 'Token "李四-前端" 剩余额度不足 20%', 'warning'),
            ('provider_down', '数据源 Custom Gateway 已断开', 'critical'),
            ('error_rate', 'Qwen 模型错误率异常（8.5%）', 'warning'),
            ('provider_down', 'OpenRouter 响应超时（>30s）', 'warning'),
            ('error_rate', 'claude-sonnet-4 错误率上升（5.2%）', 'warning'),
            ('quota_remaining', 'Token "孙七-后端" 剩余额度不足 10%', 'critical'),
            ('error_rate', 'deepseek-reasoner 错误率超过阈值', 'critical'),
            ('provider_down', 'OpenAI 临时不可用（5分钟内恢复）', 'warning'),
            ('quota_remaining', '组织 "LLM Ops" 月度额度已用 85%', 'warning'),
            ('error_rate', '通用错误率恢复正常', 'info'),
            ('provider_down', 'Custom Gateway 恢复连接', 'info'),
            ('quota_remaining', 'Token "周八-ML" 已自动补充额度', 'info'),
        ]
        for i, (_, msg, level) in enumerate(alert_msgs):
            h = AlertHistory(
                rule_id=None,
                event_data=json.dumps({'source': 'demo_seed', 'seq': i}),
                message=msg,
                level=level,
                channels_sent=json.dumps(['wechat'] if level == 'critical' else ['email']),
                created_at=now - timedelta(hours=random.randint(1, 72)),
            )
            db.session.add(h)

    # ---- 7. Channel Health ----
    if ChannelHealth.query.count() == 0:
        provider_names = [p['name'] for p in _DEMO_PROVIDERS]
        statuses = ['healthy', 'healthy', 'healthy', 'healthy', 'warning', 'dead']
        for day_offset in range(14):
            for pname in provider_names:
                provider = provider_map.get(pname)
                if not provider:
                    continue
                # 随机状态，但最近的更可能健康
                base_healthy = 0.8 if day_offset < 3 else 0.6
                if pname == 'Custom Gateway':
                    st = random.choices(['dead', 'healthy', 'unhealthy'], weights=[0.7, 0.1, 0.2])[0]
                elif pname == 'OpenRouter':
                    st = random.choices(statuses, weights=[0.4, 0.2, 0.1, 0.1, 0.15, 0.05])[0]
                else:
                    st = random.choices(['healthy', 'healthy', 'healthy', 'healthy', 'warning', 'dead'],
                                        weights=[base_healthy, base_healthy, base_healthy * 0.5, 0.2, 0.15, 0.05])[0]
                lat = random.randint(50, 2000) if st == 'healthy' else random.randint(3000, 15000)
                h = ChannelHealth(
                    provider_id=provider.id,
                    status=st,
                    latency_ms=lat,
                    error_message='' if st == 'healthy' else f'Sample error: {st}',
                    checked_at=now - timedelta(days=day_offset, hours=random.randint(0, 23)),
                )
                db.session.add(h)

    # ---- 8. Desensitize Rules ----
    for dd in _DEMO_DESENSITIZE_RULES:
        existing = DesensitizeRule.query.filter_by(name=dd['name']).first()
        if not existing:
            rule = DesensitizeRule(
                name=dd['name'], type=dd['type'], pattern=dd['pattern'],
                category=dd['category'], level=dd['level'], enabled=True,
            )
            db.session.add(rule)

    # ---- 9. Model Aliases ----
    for md in _DEMO_MODEL_ALIASES:
        existing = ModelAlias.query.filter_by(alias=md['alias']).first()
        if not existing:
            a = ModelAlias(alias=md['alias'], target=md['target'], enabled=True)
            db.session.add(a)

    # ---- 10. RequestLog (~2000, 30天分布) ----
    if RequestLog.query.count() == 0:
        token_names = list(token_map.keys())
        provider_names = [p['name'] for p in _DEMO_PROVIDERS]
        model_names = [
            'gpt-4o', 'gpt-4o-mini', 'claude-sonnet-4', 'claude-haiku-4-5',
            'deepseek-chat', 'deepseek-reasoner', 'qwen-plus', 'qwen-turbo',
        ]
        endpoints = ['/v1/chat/completions', '/v1/messages', '/v1/completions']
        status_codes = [200, 200, 200, 200, 200, 200, 200, 400, 401, 429, 500]
        error_types_map = {400: 'unknown', 401: 'auth_failed', 429: 'rate_limited', 500: 'unknown'}

        logs = []
        total_records = 2000

        for i in range(total_records):
            # 30天前到现在的随机时间
            days_ago = random.random() * 30
            # 小时分布：10点和15点有峰值
            hour_bias = random.random()
            if hour_bias < 0.25:
                hour = random.randint(9, 11)
            elif hour_bias < 0.45:
                hour = random.randint(14, 16)
            elif hour_bias < 0.6:
                hour = random.randint(8, 18)
            else:
                hour = random.randint(0, 23)

            # 工作日 vs 周末（周末请求少）
            day_ts = now - timedelta(days=days_ago)
            is_weekend = day_ts.weekday() >= 5
            if is_weekend and random.random() < 0.6:
                continue  # 跳过60%的周末请求

            token_name = random.choice(token_names)
            token = token_map[token_name]
            pname = random.choice(provider_names)
            provider = provider_map[pname]
            model = random.choice(model_names)
            endpoint = random.choice(endpoints)
            status = random.choice(status_codes)
            inp = random.randint(50, 8000)
            out = random.randint(10, 4000)
            lat = random.randint(200, 5000) if status == 200 else random.randint(100, 10000)
            cost = random.randint(0, 500)
            is_stream = random.random() < 0.4
            is_retry = random.random() < 0.05 and status != 200
            error_msg = '' if status == 200 else f'Error {status}: sample error'
            error_type = error_types_map.get(status, '')
            cache_hit = random.randint(0, inp) if status == 200 and random.random() < 0.3 else 0

            log = RequestLog(
                request_id=f'demo-{random.getrandbits(64):016x}',
                token_id=token.id,
                token_name=token_name,
                provider_id=provider.id,
                provider_name=pname,
                model_name=model,
                endpoint=endpoint,
                input_tokens=inp,
                output_tokens=out,
                status_code=status,
                latency_ms=lat,
                cost_cents=cost,
                is_stream=is_stream,
                is_retry=is_retry,
                error_message=error_msg,
                error_type=error_type,
                cached_tokens=cache_hit,
                client_ip=f'192.168.{random.randint(0, 255)}.{random.randint(1, 254)}',
                created_at=day_ts.replace(hour=hour, minute=random.randint(0, 59), second=random.randint(0, 59)),
            )
            logs.append(log)

        db.session.bulk_save_objects(logs)

    db.session.commit()

    # 统计
    provider_count = Provider.query.count()
    token_count = WRToken.query.count()
    log_count = RequestLog.query.count()
    org_count = Org.query.count()
    app.logger.info(f'[demo] 种子数据就绪: {org_count} 组织, {provider_count} 数据源, {token_count} Token, {log_count} 请求日志')