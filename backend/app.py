from flask import Flask
from extensions import db, cors
from config import get_config
import logging

logging.basicConfig(level=logging.INFO, format='%(asctime)s [%(name)s] %(levelname)s: %(message)s')


def create_app(config_class=None):
    app = Flask(__name__,
                static_folder='static',
                static_url_path='/static')

    if config_class is None:
        config_class = get_config()

    app.config.from_object(config_class)

    # 初始化扩展
    db.init_app(app)
    cors.init_app(app, resources={r"/api/*": {"origins": "*"}})

    # 注册路由蓝图
    from routes.dashboard import dashboard_bp
    from routes.monitor import monitor_bp
    from routes.alert import alert_bp
    from routes.billing import billing_bp
    from routes.team import team_bp
    from routes.cli_export import cli_bp
    from routes.settings import settings_bp

    app.register_blueprint(dashboard_bp, url_prefix='/api/dashboard')
    app.register_blueprint(monitor_bp, url_prefix='/api/monitor')
    app.register_blueprint(alert_bp, url_prefix='/api/alerts')
    app.register_blueprint(billing_bp, url_prefix='/api/billing')
    app.register_blueprint(team_bp, url_prefix='/api/team')
    app.register_blueprint(cli_bp, url_prefix='/api/cli')
    app.register_blueprint(settings_bp, url_prefix='/api/settings')

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
        from models.wr_models import AlertRule, AlertHistory, ChannelHealth, TeamQuota, CostRecord  # noqa: F401
        db.create_all()

    # 启动定时任务（仅非Debug模式或环境变量启用时）
    _init_schedulers(app)

    return app


def _init_schedulers(app):
    """初始化APScheduler定时任务"""
    import os
    # 开发模式默认不启动定时任务，避免干扰调试
    if app.debug and not os.environ.get('ENABLE_SCHEDULER'):
        app.logger.info('Debug模式，定时任务未启用 (设置 ENABLE_SCHEDULER=1 启用)')
        return

    from apscheduler.schedulers.background import BackgroundScheduler
    scheduler = BackgroundScheduler()

    # 1. 渠道健康检测（每5分钟）
    check_interval = app.config.get('HEALTH_CHECK_INTERVAL', 300)
    scheduler.add_job(
        _scheduled_health_check,
        'interval',
        seconds=check_interval,
        args=[app],
        id='health_check',
        replace_existing=True,
    )

    # 2. 统计采集（每5分钟）
    scheduler.add_job(
        _scheduled_stats_collect,
        'interval',
        minutes=5,
        args=[app],
        id='stats_collect',
        replace_existing=True,
    )

    # 3. 告警评估（每1分钟）
    scheduler.add_job(
        _scheduled_alert_evaluate,
        'interval',
        minutes=1,
        args=[app],
        id='alert_evaluate',
        replace_existing=True,
    )

    scheduler.start()
    app.logger.info(f'定时任务已启动: 健康检测/{check_interval}s, 统计采集/5min, 告警评估/1min')

    # 确保进程退出时关闭scheduler
    import atexit
    atexit.register(lambda: scheduler.shutdown(wait=False))


def _scheduled_health_check(app):
    """定时健康检测任务"""
    with app.app_context():
        try:
            from services.health_checker import HealthChecker
            checker = HealthChecker()
            results = checker.check_all_sync()
            app.logger.info(f'健康检测完成: {len(results)}个渠道')
        except Exception as e:
            app.logger.error(f'健康检测失败: {e}')


def _scheduled_stats_collect(app):
    """定时统计采集任务"""
    with app.app_context():
        try:
            from models.newapi_adapter import NewAPIAdapter
            from models.wr_models import CostRecord
            from extensions import db
            from sqlalchemy import func as sql_func
            from datetime import datetime

            raw = NewAPIAdapter.get_usage_stats(hours=1)
            if not raw:
                return

            # 写入成本记录
            for r in raw:
                inp = r.get('input_tokens', 0) or 0
                out = r.get('output_tokens', 0) or 0
                cost_cents = int(inp * 0.003 + out * 0.015)
                if inp == 0 and out == 0:
                    continue
                db.session.add(CostRecord(
                    channel_id=r.get('channel_id'),
                    model_name=r.get('model_name', 'unknown'),
                    input_tokens=inp,
                    output_tokens=out,
                    cost_cents=max(cost_cents, 0),
                    recorded_at=datetime.utcnow(),
                ))
            db.session.commit()
            app.logger.info(f'统计采集完成: {len(raw)}条')
        except Exception as e:
            app.logger.error(f'统计采集失败: {e}')


def _scheduled_alert_evaluate(app):
    """定时告警评估任务"""
    with app.app_context():
        try:
            from services.alert_engine import AlertEngine
            from models.wr_models import ChannelHealth, AlertRule
            from extensions import db

            # 如果没有启用规则，跳过
            if AlertRule.query.filter_by(enabled=True).count() == 0:
                return

            engine = AlertEngine(app=app)

            # 从最近健康检测结果构造事件
            latest = db.session.query(ChannelHealth).order_by(
                ChannelHealth.checked_at.desc()
            ).limit(20).all()

            for h in latest:
                event = {
                    'channel_id': h.channel_id,
                    'status': h.status,
                    'key_id': h.channel_id,
                }
                # 如果状态异常，触发评估
                if h.status in ('dead', 'auth_failed', 'unhealthy'):
                    event['status'] = 'failed'
                    engine.evaluate_event(event)

        except Exception as e:
            app.logger.error(f'告警评估失败: {e}')


if __name__ == '__main__':
    app = create_app()
    app.run(host='0.0.0.0', port=5000, debug=True)
