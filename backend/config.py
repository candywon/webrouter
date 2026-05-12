import os


class Config:
    """基础配置"""
    SECRET_KEY = os.environ.get('SECRET_KEY', os.urandom(32).hex())
    _default_db = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'data', 'webrouter.db')
    SQLALCHEMY_DATABASE_URI = os.environ.get('DATABASE_URI', f'sqlite:///{_default_db}')
    SQLALCHEMY_TRACK_MODIFICATIONS = False
    NEWAPI_URL = os.environ.get('NEWAPI_URL', 'http://localhost:3000')
    NEWAPI_ADMIN_TOKEN = os.environ.get('NEWAPI_ADMIN_TOKEN', '')
    REDIS_URL = os.environ.get('REDIS_URL', 'redis://localhost:6379/0')
    TZ = os.environ.get('TZ', 'Asia/Shanghai')

    # 监控配置
    HEALTH_CHECK_INTERVAL = int(os.environ.get('HEALTH_CHECK_INTERVAL', '300'))
    BALANCE_CHECK_INTERVAL = int(os.environ.get('BALANCE_CHECK_INTERVAL', '1800'))

    # 告警配置
    ALERT_COOLDOWN = int(os.environ.get('ALERT_COOLDOWN', '300'))


class DevelopmentConfig(Config):
    DEBUG = True


class ProductionConfig(Config):
    DEBUG = False


config_map = {
    'development': DevelopmentConfig,
    'production': ProductionConfig,
    'default': DevelopmentConfig,
}


def get_config():
    env = os.environ.get('FLASK_ENV', 'default')
    return config_map.get(env, DevelopmentConfig)
