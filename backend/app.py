"""WebRouter Flask 应用工厂"""
from flask import Flask
from extensions import db, cors
from config import get_config
import logging

logging.basicConfig(level=logging.INFO, format='%(asctime)s [%(name)s] %(levelname)s: %(message)s')

# 知识库8个初始业务域种子数据
_KNOWLEDGE_DOMAINS = [
    ('legal', '法务', '法务部'),
    ('finance', '财务', '财务部'),
    ('hr', '人力资源', '人事部'),
    ('admin', '行政', '行政部'),
    ('sales', '销售', '销售部'),
    ('service', '客服', '客服部'),
    ('tech', '技术', '技术部'),
    ('strategy', '战略', '战略部'),
]


def seed_knowledge_domains():
    """插入初始知识库业务域（仅首次）"""
    try:
        from models.knowledge import KnowledgeDomain, KnowledgeDomainRisk
        count = 0
        for code, name, dept in _KNOWLEDGE_DOMAINS:
            existing = KnowledgeDomain.query.filter_by(domain_code=code).first()
            if not existing:
                d = KnowledgeDomain(domain_code=code, domain_name=name, department=dept, status='active')
                db.session.add(d)
                count += 1
        db.session.commit()

        # 领域风险配置种子
        risk_defaults = {
            'legal': ('high', 'verified', 90, '【注意】以下法务信息仅供参考，不构成法律意见。', True, False, True),
            'finance': ('high', 'verified', 90, '【注意】以下财务数据仅供参考，正式报告以财务部官方数据为准。', True, True, False),
            'hr': ('medium', 'auto', 180, '【提示】以下人事信息请以最新公司制度为准。', True, False, True),
            'admin': ('medium', 'auto', 180, '【提示】以下行政信息请以最新公司制度为准。', True, False, True),
            'strategy': ('medium', 'auto', 180, '【提示】以下战略信息供内部参考。', False, True, False),
            'sales': ('low', 'auto', 365, '', True, True, True),
            'service': ('low', 'auto', 365, '', True, True, True),
            'tech': ('low', 'auto', 365, '', True, True, True),
        }
        for code, (risk, min_ver, age, disclaimer, factual, analytical, procedural) in risk_defaults.items():
            existing = KnowledgeDomainRisk.query.get(code)
            if not existing:
                r = KnowledgeDomainRisk(
                    domain_code=code, risk_level=risk, min_verification=min_ver,
                    max_age_days=age, disclaimer_template=disclaimer,
                    allow_factual_injection=factual, allow_analytical_injection=analytical,
                    allow_procedural_injection=procedural,
                )
                db.session.add(r)
                count += 1
        db.session.commit()
        return count
    except Exception:
        db.session.rollback()
        return 0


def create_app(config_class=None):
    app = Flask(__name__,
                static_folder='static',
                static_url_path='/static')

    if config_class is None:
        config_class = get_config()

    app.config.from_object(config_class)

    # 初始化扩展
    db.init_app(app)
    cors.init_app(app, resources={r"/api/*": {r"origins": "*"}})

    # 注册路由蓝图
    from routes.dashboard import dashboard_bp
    from routes.providers import providers_bp
    from routes.monitor import monitor_bp
    from routes.alert import alert_bp
    from routes.billing import billing_bp
    from routes.team import team_bp
    from routes.cli_export import cli_bp
    from routes.settings import settings_bp
    from routes.tokens import tokens_bp       # Token 管理
    from routes.pricing import pricing_bp     # 模型定价管理
    from routes.channel import channel_bp    # Provider 渠道管理
    from routes.desensitize import desensitize_bp  # 脱敏规则管理
    from routes.modelgrades import modelgrades_bp  # 模型分级管理
    from routes.modelaliases import modelaliases_bp  # 模型别名管理
    from routes.knowledge_routes import knowledge_bp  # 企业知识库

    app.register_blueprint(dashboard_bp, url_prefix='/api/dashboard')
    app.register_blueprint(providers_bp, url_prefix='/api/providers')
    app.register_blueprint(monitor_bp, url_prefix='/api/monitor')
    app.register_blueprint(alert_bp, url_prefix='/api/alerts')
    app.register_blueprint(billing_bp, url_prefix='/api/billing')
    app.register_blueprint(team_bp, url_prefix='/api/team')
    app.register_blueprint(cli_bp, url_prefix='/api/cli')
    app.register_blueprint(settings_bp, url_prefix='/api/settings')
    app.register_blueprint(tokens_bp, url_prefix='/api/tokens')
    app.register_blueprint(pricing_bp, url_prefix='/api/pricing')
    app.register_blueprint(channel_bp, url_prefix='/api/providers')
    app.register_blueprint(desensitize_bp, url_prefix='/api/desensitize')
    app.register_blueprint(modelgrades_bp, url_prefix='/api/modelgrades')
    app.register_blueprint(modelaliases_bp, url_prefix='/api/modelaliases')
    app.register_blueprint(knowledge_bp, url_prefix='/api/knowledge')

    # 根路径返回前端页面
    @app.route('/')
    def index():
        return app.send_static_file('index.html')

    # 健康检查
    @app.route('/health')
    def health():
        return {'status': 'ok', 'service': 'webrouter'}

    # 初始化数据库
    with app.app_context():
        from models.wr_models import (  # noqa: F401
            WRToken, ProviderExt, ProviderQuota, RequestLog,
            AlertRule, AlertHistory, ChannelHealth, TeamQuota,
            SystemSetting, ModelGrade, ModelAlias,
        )
        from models.provider import Provider  # noqa: F401
        from models.knowledge import (  # noqa: F401
            KnowledgeRaw, KnowledgeItem, KnowledgeDomain,
            KnowledgeDomainRisk, KnowledgeAnalysis,
        )
        db.create_all()

        # 知识库字段迁移：为已存在的 wr_tokens 表添加新列
        from sqlalchemy import text
        knowledge_cols = [
            ('knowledge_capture_enabled', 'BOOLEAN DEFAULT 0'),
            ('knowledge_department', 'VARCHAR(100) DEFAULT \'\''),
            ('rag_enabled', 'BOOLEAN DEFAULT 0'),
            ('rag_min_relevance', 'FLOAT DEFAULT 0.7'),
            ('rag_top_k', 'INTEGER DEFAULT 3'),
            ('system_prompt_knowledge', 'TEXT DEFAULT \'\''),
        ]
        for col, ctype in knowledge_cols:
            try:
                db.session.execute(text(f'ALTER TABLE wr_tokens ADD COLUMN {col} {ctype}'))
            except Exception:
                pass  # 列已存在
        db.session.commit()

        # 初始化种子数据（仅首次建表）
        from models.wr_models import ModelPricing, SystemSetting, ModelGrade
        count1 = ModelPricing.seed_defaults()
        if count1:
            app.logger.info(f'定价种子数据已初始化: {count1} 条')
        count2 = SystemSetting.seed_defaults()
        if count2:
            app.logger.info(f'系统设置种子数据已初始化: {count2} 条')
        count3 = ModelGrade.seed_defaults()
        if count3:
            app.logger.info(f'模型分级种子数据已初始化: {count3} 条')
        count4 = ModelAlias.seed_defaults()
        if count4:
            app.logger.info(f'模型别名种子数据已初始化: {count4} 条')
        count5 = seed_knowledge_domains()
        if count5:
            app.logger.info(f'知识库域种子数据已初始化: {count5} 条')

    # 启动定时任务
    _init_schedulers(app)

    return app


def _init_schedulers(app):
    """初始化定时任务"""
    import os
    if app.debug and not os.environ.get('ENABLE_SCHEDULER'):
        app.logger.info('Debug模式，定时任务未启用 (设置 ENABLE_SCHEDULER=1 启用)')
        return

    from apscheduler.schedulers.background import BackgroundScheduler
    scheduler = BackgroundScheduler()

    # 1. Provider 健康检测（每5分钟）
    check_interval = app.config.get('HEALTH_CHECK_INTERVAL', 300)
    scheduler.add_job(
        _scheduled_health_check,
        'interval',
        seconds=check_interval,
        args=[app],
        id='health_check',
        replace_existing=True,
    )

    # 2. 告警评估（每1分钟）
    scheduler.add_job(
        _scheduled_alert_evaluate,
        'interval',
        minutes=1,
        args=[app],
        id='alert_evaluate',
        replace_existing=True,
    )

    scheduler.start()
    app.logger.info(f'定时任务已启动: 健康检测/{check_interval}s, 告警评估/1min')

    import atexit
    atexit.register(lambda: scheduler.shutdown(wait=False))


def _scheduled_health_check(app):
    """定时健康检测"""
    with app.app_context():
        try:
            from services.health_checker import HealthChecker
            checker = HealthChecker()
            results = checker.check_all_providers()
            app.logger.info(f'Provider健康检测完成: {len(results)}个数据源')
        except Exception as e:
            app.logger.error(f'健康检测失败: {e}')


def _scheduled_alert_evaluate(app):
    """定时告警评估"""
    with app.app_context():
        try:
            from services.alert_engine import AlertEngine
            from models.wr_models import ChannelHealth, AlertRule, SystemSetting
            from extensions import db

            if AlertRule.query.filter_by(enabled=True).count() == 0:
                return

            engine = AlertEngine(app=app)

            # 读取告警通道配置
            channel_config = {}
            wechat_sendkey = SystemSetting.get('alert_wechat_sendkey', '')
            if wechat_sendkey:
                channel_config['wechat'] = {'sendkey': wechat_sendkey}

            smtp_host = SystemSetting.get('alert_smtp_host', '')
            email_to = SystemSetting.get('alert_email_to', '')
            smtp_port = SystemSetting.get('alert_smtp_port', 587)
            if smtp_host and email_to:
                channel_config['email'] = {
                    'to_addr': email_to,
                    'smtp_host': smtp_host,
                    'smtp_port': smtp_port,
                    'smtp_user': SystemSetting.get('alert_smtp_user', ''),
                    'smtp_password': SystemSetting.get('alert_smtp_password', ''),
                    'smtp_use_tls': smtp_port != 465,
                    'from_addr': SystemSetting.get('alert_smtp_from', ''),
                }

            latest = db.session.query(ChannelHealth).order_by(
                ChannelHealth.checked_at.desc()
            ).limit(20).all()

            for h in latest:
                if h.status in ('dead', 'auth_failed', 'unhealthy'):
                    event = {
                        'channel_id': h.channel_id,
                        'provider_id': h.provider_id,
                        'status': 'failed',
                    }
                    engine.evaluate_event(event, channel_config=channel_config)

        except Exception as e:
            app.logger.error(f'告警评估失败: {e}')


if __name__ == '__main__':
    import os
    app = create_app()
    port = int(os.environ.get('WR_PORT', 5050))
    app.run(host='0.0.0.0', port=port, debug=True)
