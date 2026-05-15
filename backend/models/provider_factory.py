"""Provider 适配器工厂 — 根据类型创建对应的适配器实例"""
import time
import logging
import requests as http

logger = logging.getLogger(__name__)


class BaseProviderAdapter:
    """Provider 适配器基类"""

    PROVIDER_TYPE = None

    def __init__(self, provider: dict):
        self.provider = provider
        self.base_url = (provider.get('base_url') or '').rstrip('/')
        self.api_key = provider.get('api_key', '')

    def check_health(self) -> dict:
        raise NotImplementedError

    def get_models(self) -> list:
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

    def _send_test_request(self, endpoint, body, headers=None, timeout=15):
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
            result['error'] = 'Request timeout'
        except http.ConnectionError:
            result['status'] = 'dead'
            result['error'] = 'Connection refused'
        except Exception as e:
            result['status'] = 'dead'
            result['error'] = str(e)[:200]

        return result


class DirectProviderAdapter(BaseProviderAdapter):
    """直连官方 API"""

    PROVIDER_TYPE = 'direct'

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
        from urllib.parse import urlparse
        parsed = urlparse(self.base_url)
        host = parsed.hostname or ''

        config = None
        for domain, cfg in self.VENDOR_CONFIGS.items():
            if domain in host:
                config = cfg
                break

        if not config:
            config = {
                'endpoint': '/v1/chat/completions',
                'body': {'model': 'gpt-4o-mini', 'messages': [{'role': 'user', 'content': 'hi'}], 'max_tokens': 1},
            }

        # 优先用 Provider 实际配置的模型来检测，而非硬编码模型名
        body = dict(config['body'])
        provider_models = self.get_models()
        if provider_models:
            body['model'] = provider_models[0]

        # 智能去重：如果 base_url 路径已包含 endpoint 前缀，则去掉重复部分
        # 例：base_url=.../compatible-mode/v1 + endpoint=/compatible-mode/v1/chat/completions
        #   → 实际 endpoint=/chat/completions
        endpoint = config['endpoint']
        base_path = parsed.path.rstrip('/') if parsed.path else ''
        if base_path and endpoint.startswith(base_path):
            endpoint = endpoint[len(base_path):]
            if not endpoint.startswith('/'):
                endpoint = '/' + endpoint

        headers = config.get('headers_extra', {})
        return self._send_test_request(endpoint, body, headers=headers)


class AggregateProviderAdapter(BaseProviderAdapter):
    """聚合平台"""

    PROVIDER_TYPE = 'aggregate'

    def check_health(self) -> dict:
        body = {'model': 'gpt-4o-mini', 'messages': [{'role': 'user', 'content': 'hi'}], 'max_tokens': 1}
        provider_models = self.get_models()
        if provider_models:
            body['model'] = provider_models[0]
        return self._send_test_request('/v1/chat/completions', body)


class LiteLLMProviderAdapter(BaseProviderAdapter):
    """LiteLLM 代理"""

    PROVIDER_TYPE = 'litellm'

    def __init__(self, provider: dict):
        super().__init__(provider)
        self.master_key = provider.get('master_key', '')

    def check_health(self) -> dict:
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
    """自定义网关"""

    PROVIDER_TYPE = 'custom'

    def check_health(self) -> dict:
        health_endpoint = self.provider.get('health_endpoint', '')

        if health_endpoint:
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

        body = {'model': 'gpt-4o-mini', 'messages': [{'role': 'user', 'content': 'hi'}], 'max_tokens': 1}
        provider_models = self.get_models()
        if provider_models:
            body['model'] = provider_models[0]
        return self._send_test_request('/v1/chat/completions', body)


class ProviderFactory:
    """Provider 适配器工厂"""

    _adapters = {
        'direct': DirectProviderAdapter,
        'aggregate': AggregateProviderAdapter,
        'litellm': LiteLLMProviderAdapter,
        'custom': CustomProviderAdapter,
    }

    @classmethod
    def create(cls, provider: dict) -> BaseProviderAdapter:
        provider_type = provider.get('type', 'custom')
        adapter_class = cls._adapters.get(provider_type, CustomProviderAdapter)
        return adapter_class(provider)

    @classmethod
    def get_supported_types(cls):
        return list(cls._adapters.keys())

    @classmethod
    def auto_detect_type(cls, base_url: str) -> str:
        if not base_url:
            return 'custom'

        url_lower = base_url.lower()

        official_domains = [
            'api.openai.com', 'api.anthropic.com',
            'generativelanguage.googleapis.com', 'open.bigmodel.cn',
            'dashscope.aliyuncs.com', 'api.mistral.ai',
            'api.deepseek.com', 'api.groq.com',
        ]
        for domain in official_domains:
            if domain in url_lower:
                return 'direct'

        aggregate_domains = [
            'openrouter.ai', 'anyroute', 'ohmygpt', 'api2d',
            'closeai', 'api2gpt', 'gptgod', 'aigc',
        ]
        for kw in aggregate_domains:
            if kw in url_lower:
                return 'aggregate'

        from urllib.parse import urlparse
        parsed = urlparse(base_url)
        if parsed.port == 4000:
            return 'litellm'

        try:
            resp = http.get(f"{base_url.rstrip('/')}/health", timeout=5)
            if resp.status_code == 200:
                return 'litellm'
        except Exception:
            pass

        return 'custom'
