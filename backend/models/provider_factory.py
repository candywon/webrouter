"""Provider 适配器工厂 — 根据类型创建对应的适配器实例"""
import time
import logging
import requests as http
from models.provider import Provider

logger = logging.getLogger(__name__)


class BaseProviderAdapter:
    """Provider 适配器基类 — 所有类型必须实现 check_health"""

    PROVIDER_TYPE = None

    def __init__(self, provider: dict):
        self.provider = provider
        self.base_url = (provider.get('base_url') or '').rstrip('/')
        self.api_key = provider.get('api_key', '')

    def check_health(self) -> dict:
        """健康检测 — 所有类型必须实现"""
        raise NotImplementedError

    def get_models(self) -> list:
        """获取支持的模型列表 — 可选覆盖"""
        models = self.provider.get('models')
        if isinstance(models, list):
            return models
        if isinstance(models, str):
            import json
            try:
                return json.loads(models)
            except (json.JSONDecodeError, TypeError):
                pass
        return []

    def get_channels(self) -> list:
        """获取渠道列表 — 仅 newapi/oneapi"""
        return []

    def get_usage_stats(self, hours=24) -> list:
        """获取用量统计 — 仅 newapi/oneapi"""
        return []

    def get_users(self) -> list:
        """获取用户列表 — 仅 newapi/oneapi"""
        return []

    def _send_test_request(self, endpoint, body, headers=None, timeout=15):
        """发送测试请求的通用方法"""
        result = {
            'provider_id': self.provider.get('id'),
            'name': self.provider.get('name', ''),
            'status': 'unknown',
            'latency_ms': 0,
            'error': None,
        }

        url = f"{self.base_url}{endpoint}"
        default_headers = {
            'Authorization': f'Bearer {self.api_key}',
            'Content-Type': 'application/json',
        }
        if headers:
            default_headers.update(headers)

        try:
            start = time.monotonic()
            resp = http.post(url, json=body, headers=default_headers, timeout=timeout)
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
        except http.ConnectionError:
            result['status'] = 'dead'
            result['error'] = 'Connection refused'
        except Exception as e:
            result['status'] = 'dead'
            result['error'] = str(e)[:200]

        return result


class DirectProviderAdapter(BaseProviderAdapter):
    """直连官方 API — 根据 base_url 识别厂商"""

    PROVIDER_TYPE = 'direct'

    # 官方 API 测试配置
    VENDOR_CONFIGS = {
        'api.openai.com': {
            'endpoint': '/v1/chat/completions',
            'body': {'model': 'gpt-4o-mini', 'messages': [{'role': 'user', 'content': 'hi'}], 'max_tokens': 1},
        },
        'api.anthropic.com': {
            'endpoint': '/v1/messages',
            'body': {'model': 'claude-3-haiku-20240307', 'messages': [{'role': 'user', 'content': 'hi'}], 'max_tokens': 1},
            'headers_extra': {'anthropic-version': '2023-06-01'},
        },
        'generativelanguage.googleapis.com': {
            'endpoint': '/v1beta/models/gemini-2.0-flash:generateContent',
            'body': {'contents': [{'parts': [{'text': 'hi'}]}]},
        },
        'open.bigmodel.cn': {
            'endpoint': '/v4/chat/completions',
            'body': {'model': 'glm-4-flash', 'messages': [{'role': 'user', 'content': 'hi'}], 'max_tokens': 1},
        },
        'dashscope.aliyuncs.com': {
            'endpoint': '/compatible-mode/v1/chat/completions',
            'body': {'model': 'qwen-turbo', 'messages': [{'role': 'user', 'content': 'hi'}], 'max_tokens': 1},
        },
    }

    def check_health(self) -> dict:
        # 根据 base_url 匹配厂商
        from urllib.parse import urlparse
        parsed = urlparse(self.base_url)
        host = parsed.hostname or ''

        config = None
        for domain, cfg in self.VENDOR_CONFIGS.items():
            if domain in host:
                config = cfg
                break

        if not config:
            # 未知厂商，走 OpenAI 兼容格式
            config = {
                'endpoint': '/v1/chat/completions',
                'body': {'model': 'gpt-4o-mini', 'messages': [{'role': 'user', 'content': 'hi'}], 'max_tokens': 1},
            }

        headers = config.get('headers_extra', {})
        return self._send_test_request(config['endpoint'], config['body'], headers=headers)


class AggregateProviderAdapter(BaseProviderAdapter):
    """聚合平台 — 通常兼容 OpenAI 格式"""

    PROVIDER_TYPE = 'aggregate'

    def check_health(self) -> dict:
        # 聚合平台基本兼容 OpenAI 格式
        body = {'model': 'gpt-4o-mini', 'messages': [{'role': 'user', 'content': 'hi'}], 'max_tokens': 1}
        return self._send_test_request('/v1/chat/completions', body)


class NewAPIProviderAdapter(BaseProviderAdapter):
    """自建 New-API — 功能最丰富的 Provider 类型"""

    PROVIDER_TYPE = 'newapi'

    def __init__(self, provider: dict):
        super().__init__(provider)
        self.admin_token = provider.get('admin_token', '')
        self.db_uri = provider.get('db_uri', '')
        self._db_adapter = None

    def _get_db_adapter(self):
        """懒加载 New-API 数据库适配器"""
        if self._db_adapter is None and self.db_uri:
            try:
                from models.newapi_adapter import NewAPIAdapter
                self._db_adapter = NewAPIAdapter()
            except Exception as e:
                logger.warning(f"Failed to init NewAPI adapter for {self.base_url}: {e}")
        return self._db_adapter

    def check_health(self) -> dict:
        # 优先通过 HTTP 检测 /api/status
        result = {
            'provider_id': self.provider.get('id'),
            'name': self.provider.get('name', ''),
            'status': 'unknown',
            'latency_ms': 0,
            'error': None,
        }

        try:
            start = time.monotonic()
            headers = {}
            if self.admin_token:
                headers['Authorization'] = f'Bearer {self.admin_token}'
            resp = http.get(
                f"{self.base_url}/api/status",
                headers=headers,
                timeout=10,
            )
            result['latency_ms'] = int((time.monotonic() - start) * 1000)

            if resp.status_code == 200:
                result['status'] = 'healthy'
            elif resp.status_code in (401, 403):
                result['status'] = 'auth_failed'
            else:
                result['status'] = 'unhealthy'
                result['error'] = f'HTTP {resp.status_code}'

        except http.Timeout:
            result['status'] = 'timeout'
            result['error'] = 'Request timeout (10s)'
        except http.ConnectionError:
            result['status'] = 'dead'
            result['error'] = 'Connection refused'
        except Exception as e:
            result['status'] = 'dead'
            result['error'] = str(e)[:200]

        return result

    def get_channels(self) -> list:
        adapter = self._get_db_adapter()
        if adapter:
            try:
                return adapter.get_channels()
            except Exception as e:
                logger.warning(f"Failed to get channels: {e}")
        return []

    def get_usage_stats(self, hours=24) -> list:
        adapter = self._get_db_adapter()
        if adapter:
            try:
                return adapter.get_usage_stats(hours)
            except Exception as e:
                logger.warning(f"Failed to get usage stats: {e}")
        return []

    def get_users(self) -> list:
        adapter = self._get_db_adapter()
        if adapter:
            try:
                return adapter.get_users()
            except Exception as e:
                logger.warning(f"Failed to get users: {e}")
        return []


class OneAPIProviderAdapter(NewAPIProviderAdapter):
    """自建 One-API — 与 New-API 共享 DB 结构"""

    PROVIDER_TYPE = 'oneapi'

    def check_health(self) -> dict:
        # One-API 的状态端点
        result = {
            'provider_id': self.provider.get('id'),
            'name': self.provider.get('name', ''),
            'status': 'unknown',
            'latency_ms': 0,
            'error': None,
        }

        try:
            start = time.monotonic()
            resp = http.get(f"{self.base_url}/api/status", timeout=10)
            result['latency_ms'] = int((time.monotonic() - start) * 1000)

            if resp.status_code == 200:
                result['status'] = 'healthy'
            elif resp.status_code in (401, 403):
                result['status'] = 'auth_failed'
            else:
                result['status'] = 'unhealthy'
                result['error'] = f'HTTP {resp.status_code}'

        except http.Timeout:
            result['status'] = 'timeout'
            result['error'] = 'Request timeout (10s)'
        except http.ConnectionError:
            result['status'] = 'dead'
            result['error'] = 'Connection refused'
        except Exception as e:
            result['status'] = 'dead'
            result['error'] = str(e)[:200]

        return result


class LiteLLMProviderAdapter(BaseProviderAdapter):
    """LiteLLM 代理 — 支持 /v1/models 自动发现"""

    PROVIDER_TYPE = 'litellm'

    def __init__(self, provider: dict):
        super().__init__(provider)
        self.master_key = provider.get('master_key', '')

    def check_health(self) -> dict:
        # LiteLLM 通过 /health 端点检测
        result = {
            'provider_id': self.provider.get('id'),
            'name': self.provider.get('name', ''),
            'status': 'unknown',
            'latency_ms': 0,
            'error': None,
        }

        try:
            start = time.monotonic()
            headers = {}
            if self.master_key:
                headers['Authorization'] = f'Bearer {self.master_key}'
            resp = http.get(f"{self.base_url}/health", headers=headers, timeout=10)
            result['latency_ms'] = int((time.monotonic() - start) * 1000)

            if resp.status_code == 200:
                result['status'] = 'healthy'
            else:
                # 回退到 /v1/models 检测
                return self._check_via_models()
        except Exception:
            return self._check_via_models()

        return result

    def _check_via_models(self) -> dict:
        result = {
            'provider_id': self.provider.get('id'),
            'name': self.provider.get('name', ''),
            'status': 'unknown',
            'latency_ms': 0,
            'error': None,
        }

        try:
            start = time.monotonic()
            headers = {}
            if self.master_key:
                headers['Authorization'] = f'Bearer {self.master_key}'
            resp = http.get(f"{self.base_url}/v1/models", headers=headers, timeout=10)
            result['latency_ms'] = int((time.monotonic() - start) * 1000)

            if resp.status_code == 200:
                result['status'] = 'healthy'
            elif resp.status_code in (401, 403):
                result['status'] = 'auth_failed'
            else:
                result['status'] = 'unhealthy'
                result['error'] = f'HTTP {resp.status_code}'

        except http.Timeout:
            result['status'] = 'timeout'
        except http.ConnectionError:
            result['status'] = 'dead'
            result['error'] = 'Connection refused'
        except Exception as e:
            result['status'] = 'dead'
            result['error'] = str(e)[:200]

        return result

    def get_models(self) -> list:
        """通过 /v1/models 自动发现模型"""
        try:
            headers = {}
            if self.master_key:
                headers['Authorization'] = f'Bearer {self.master_key}'
            resp = http.get(f"{self.base_url}/v1/models", headers=headers, timeout=10)
            if resp.status_code == 200:
                data = resp.json()
                model_list = data.get('data', [])
                return [m.get('id', '') for m in model_list if m.get('id')]
        except Exception as e:
            logger.warning(f"Failed to get LiteLLM models: {e}")
        return super().get_models()


class CustomProviderAdapter(BaseProviderAdapter):
    """自定义网关 — 使用自定义健康端点或回退到 OpenAI 兼容格式"""

    PROVIDER_TYPE = 'custom'

    def check_health(self) -> dict:
        health_endpoint = self.provider.get('health_endpoint', '')

        if health_endpoint:
            # 使用自定义健康端点
            result = {
                'provider_id': self.provider.get('id'),
                'name': self.provider.get('name', ''),
                'status': 'unknown',
                'latency_ms': 0,
                'error': None,
            }

            try:
                start = time.monotonic()
                headers = {'Authorization': f'Bearer {self.api_key}'} if self.api_key else {}
                resp = http.get(f"{self.base_url}{health_endpoint}", headers=headers, timeout=10)
                result['latency_ms'] = int((time.monotonic() - start) * 1000)

                if resp.status_code == 200:
                    result['status'] = 'healthy'
                else:
                    result['status'] = 'unhealthy'
                    result['error'] = f'HTTP {resp.status_code}'
            except Exception as e:
                result['status'] = 'dead'
                result['error'] = str(e)[:200]

            return result

        # 回退到 OpenAI 兼容格式
        body = {'model': 'gpt-4o-mini', 'messages': [{'role': 'user', 'content': 'hi'}], 'max_tokens': 1}
        return self._send_test_request('/v1/chat/completions', body)


class ProviderFactory:
    """Provider 适配器工厂 — 根据类型创建适配器"""

    _adapters = {
        'direct': DirectProviderAdapter,
        'aggregate': AggregateProviderAdapter,
        'newapi': NewAPIProviderAdapter,
        'oneapi': OneAPIProviderAdapter,
        'litellm': LiteLLMProviderAdapter,
        'custom': CustomProviderAdapter,
    }

    @classmethod
    def create(cls, provider: dict) -> BaseProviderAdapter:
        """根据 Provider 类型创建对应的适配器实例"""
        provider_type = provider.get('type', 'custom')
        adapter_class = cls._adapters.get(provider_type, CustomProviderAdapter)
        return adapter_class(provider)

    @classmethod
    def get_supported_types(cls):
        """获取所有支持的 Provider 类型"""
        return list(cls._adapters.keys())

    @classmethod
    def auto_detect_type(cls, base_url: str) -> str:
        """根据 Base URL 自动检测 Provider 类型"""
        if not base_url:
            return 'custom'

        url_lower = base_url.lower()

        # 检测已知官方 API
        official_domains = [
            'api.openai.com', 'api.anthropic.com',
            'generativelanguage.googleapis.com', 'open.bigmodel.cn',
            'dashscope.aliyuncs.com', 'api.mistral.ai',
            'api.deepseek.com', 'api.groq.com',
        ]
        for domain in official_domains:
            if domain in url_lower:
                return 'direct'

        # 检测已知聚合平台
        aggregate_domains = [
            'openrouter.ai', 'anyroute', 'ohmygpt', 'api2d',
            'closeai', 'api2gpt', 'gptgod', 'aigc',
        ]
        for kw in aggregate_domains:
            if kw in url_lower:
                return 'aggregate'

        # 检测 LiteLLM（常见端口 4000）
        from urllib.parse import urlparse
        parsed = urlparse(base_url)
        if parsed.port == 4000:
            return 'litellm'

        # 尝试探测 New-API / One-API 特征端点
        try:
            resp = http.get(f"{base_url.rstrip('/')}/api/status", timeout=5)
            if resp.status_code == 200:
                data = resp.json() if resp.headers.get('content-type', '').startswith('application/json') else {}
                if isinstance(data, dict) and data.get('success') is True:
                    return 'newapi'
        except Exception:
            pass

        # 尝试探测 LiteLLM /health
        try:
            resp = http.get(f"{base_url.rstrip('/')}/health", timeout=5)
            if resp.status_code == 200:
                return 'litellm'
        except Exception:
            pass

        return 'custom'
