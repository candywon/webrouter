"""Mock演示数据 — New-API未接入时返回模拟数据"""
import random
from datetime import datetime, timedelta

DEMO_MODE = True  # 当New-API不可用时自动切换

# 模拟渠道
DEMO_CHANNELS = [
    {'id': 1, 'name': 'OpenAI 主力', 'type': 1, 'status': 1, 'priority': 10,
     'models': 'gpt-4o,gpt-4o-mini,o1-mini', 'balance': 85.32},
    {'id': 2, 'name': 'Claude 渠道', 'type': 14, 'status': 1, 'priority': 9,
     'models': 'claude-sonnet-4-20250514,claude-3-5-haiku', 'balance': 42.18},
    {'id': 3, 'name': 'Gemini 渠道', 'type': 24, 'status': 1, 'priority': 7,
     'models': 'gemini-2.0-flash,gemini-2.5-pro', 'balance': 120.00},
    {'id': 4, 'name': 'DeepSeek 渠道', 'type': 33, 'status': 1, 'priority': 6,
     'models': 'deepseek-chat,deepseek-reasoner', 'balance': 15.67},
    {'id': 5, 'name': '备用 OpenAI', 'type': 1, 'status': 0, 'priority': 3,
     'models': 'gpt-4o-mini', 'balance': 0},
    {'id': 6, 'name': '已过期 Key', 'type': 14, 'status': 2, 'priority': 1,
     'models': 'claude-3-5-haiku', 'balance': 0},
]

# 模拟健康状态
DEMO_HEALTH = {
    1: {'status': 'healthy', 'latency_ms': 230, 'error_message': None, 'checked_at': datetime.utcnow().isoformat()},
    2: {'status': 'healthy', 'latency_ms': 380, 'error_message': None, 'checked_at': datetime.utcnow().isoformat()},
    3: {'status': 'warning', 'latency_ms': 1200, 'error_message': '响应偏慢', 'checked_at': (datetime.utcnow() - timedelta(minutes=5)).isoformat()},
    4: {'status': 'healthy', 'latency_ms': 150, 'error_message': None, 'checked_at': (datetime.utcnow() - timedelta(minutes=3)).isoformat()},
    5: {'status': 'disabled', 'latency_ms': None, 'error_message': '已手动禁用', 'checked_at': (datetime.utcnow() - timedelta(hours=2)).isoformat()},
    6: {'status': 'dead', 'latency_ms': None, 'error_message': 'HTTP 401: Invalid API Key', 'checked_at': (datetime.utcnow() - timedelta(minutes=30)).isoformat()},
}


def get_demo_overview():
    """模拟仪表盘总览数据"""
    return {
        'channels': {
            'total': 6,
            'active': 4,
            'healthy': 3,
        },
        'usage': {
            'today_requests': 12847,
            'error_rate': 1.8,
        },
        'cost': {
            'month_cents': 263300,
            'month_yuan': 2633.00,
        },
    }


def get_demo_trends(days=7):
    """模拟调用趋势数据"""
    data = []
    for i in range(days):
        d = datetime.utcnow() - timedelta(days=days - 1 - i)
        data.append({
            'date': d.strftime('%m-%d'),
            'request_count': random.randint(8000, 18000),
            'input_tokens': random.randint(2000000, 6000000),
            'output_tokens': random.randint(500000, 2000000),
            'error_count': random.randint(20, 200),
        })
    return data


def get_demo_channels():
    """模拟渠道列表+健康状态"""
    result = []
    for ch in DEMO_CHANNELS:
        entry = dict(ch)
        entry['health'] = DEMO_HEALTH.get(ch['id'], {'status': 'unchecked'})
        result.append(entry)
    return result


def get_demo_cost(days=30):
    """模拟成本分析数据"""
    models = [
        ('gpt-4o', 3, 15),
        ('gpt-4o-mini', 0.15, 0.6),
        ('claude-sonnet-4-20250514', 3, 15),
        ('claude-3-5-haiku', 0.8, 4),
        ('gemini-2.0-flash', 0.075, 0.3),
        ('deepseek-chat', 0.14, 0.28),
    ]
    data = []
    for model_name, input_price, output_price in models:
        inp = random.randint(500000, 5000000)
        out = random.randint(100000, 2000000)
        cost = round(inp / 1000000 * input_price * 100 + out / 1000000 * output_price * 100)
        data.append({
            'model_name': model_name,
            'input_tokens': inp,
            'output_tokens': out,
            'cost_cents': cost,
            'cost_yuan': round(cost / 100, 2),
        })
    return data


def get_demo_team():
    """模拟团队成员数据"""
    return [
        {'id': 1, 'username': 'admin', 'display_name': '管理员', 'role': 'admin',
         'quota': {'quota_total': 10000000, 'quota_used': 7230000, 'quota_remaining': 2770000}},
        {'id': 2, 'username': 'dev_zhang', 'display_name': '张工', 'role': 'member',
         'quota': {'quota_total': 3000000, 'quota_used': 1850000, 'quota_remaining': 1150000}},
        {'id': 3, 'username': 'dev_li', 'display_name': '李工', 'role': 'member',
         'quota': {'quota_total': 2000000, 'quota_used': 2100000, 'quota_remaining': 0}},
    ]


def get_demo_settings():
    """模拟系统设置"""
    return {
        'newapi_url': 'http://new-api:3000',
        'health_check_interval': 300,
        'alert_cooldown': 600,
        'timezone': 'Asia/Shanghai',
        'demo_mode': True,
    }
