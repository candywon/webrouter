from flask import Flask
from extensions import db, cors
from config import get_config


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

    # 初始化数据库（导入模型确保create_all能创建表）
    with app.app_context():
        from models.wr_models import AlertRule, AlertHistory, ChannelHealth, TeamQuota, CostRecord  # noqa: F401
        db.create_all()

    return app


if __name__ == '__main__':
    app = create_app()
    app.run(host='0.0.0.0', port=5000, debug=True)
