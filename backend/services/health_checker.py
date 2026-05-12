"""渠道健康检测服务"""
import time
import logging
import requests as http
from extensions import db
from models.wr_models import ChannelHealth
from models.newapi_adapter import NewAPIAdapter

logger = logging.getLogger(__name__)

# 每种渠道类型的测试请求模板
TEST_REQUESTS = {
    1: {  # OpenAI
        'endpoint': '/v1/chat/completions',
        'body': {
            'model': 'gpt-4o-mini',
            'messages': [{'role': 'user', 'content': 'hi'}],
            'max_tokens': 1,
        },
    },
    14: {  # Anthropic
        'endpoint': '/v1/messages',
        'body': {
            'model': 'claude-3-haiku-20240307',
            'messages': [{'role': 'user', 'content': 'hi'}],
            'max_tokens': 1,
        },
        'headers_extra': {'anthropic-version': '2023-06-01'},
    },
    24: {  # Gemini
        'endpoint': '/v1/chat/completions',
        'body': {
            'model': 'gemini-2.0-flash',
            'messages': [{'role': 'user', 'content': 'hi'}],
            'max_tokens': 1,
        },
    },
}


class HealthChecker:
    """渠道健康检测引擎"""

    def check_channel_sync(self, channel: dict) -> dict:
        """同步检测单个渠道"""
        result = {
            'channel_id': channel.get('id'),
            'name': channel.get('name', ''),
            'status': 'unknown',
            'latency_ms': 0,
            'error': None,
        }

        base_url = channel.get('base_url', '')
        if not base_url:
            result['status'] = 'dead'
            result['error'] = 'No base_url configured'
            return result

        channel_type = channel.get('type', 1)
        test_conf = TEST_REQUESTS.get(channel_type, TEST_REQUESTS[1])

        # 从channel的other字段提取key
        other = channel.get('other') or ''
        api_key = other.split('\n')[0] if other else ''

        try:
            start = time.monotonic()
            headers = {
                'Authorization': f'Bearer {api_key}',
                'Content-Type': 'application/json',
            }
            if 'headers_extra' in test_conf:
                headers.update(test_conf['headers_extra'])

            resp = http.post(
                f"{base_url.rstrip('/')}{test_conf['endpoint']}",
                json=test_conf['body'],
                headers=headers,
                timeout=15,
            )
            result['latency_ms'] = int((time.monotonic() - start) * 1000)

            if resp.status_code == 200:
                result['status'] = 'healthy'
            elif resp.status_code == 429:
                result['status'] = 'rate_limited'
            elif resp.status_code in (401, 403):
                result['status'] = 'auth_failed'
            else:
                result['status'] = 'unhealthy'
                result['error'] = f'HTTP {resp.status_code}'

        except http.Timeout:
            result['status'] = 'timeout'
            result['error'] = 'Request timeout (15s)'
        except Exception as e:
            result['status'] = 'dead'
            result['error'] = str(e)[:200]

        return result

    def check_all_sync(self):
        """同步检测所有渠道"""
        try:
            channels = NewAPIAdapter.get_channels()
        except Exception as e:
            logger.error(f"Failed to get channels: {e}")
            return []

        results = []
        for ch in channels:
            result = self.check_channel_sync(ch)

            # 保存检测结果
            health = ChannelHealth(
                channel_id=ch['id'],
                status=result['status'],
                latency_ms=result.get('latency_ms'),
                error_message=result.get('error'),
            )
            db.session.add(health)
            results.append(result)

        db.session.commit()
        return results
