# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

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
        logger.debug(f"[{self.provider.get('name')}] Request URL: {url}")
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
        # 智能去重：如果 base_url 路径以 /vN 结尾，且 endpoint 也以 /vN 开头，去掉 endpoint 的版本前缀
        # 例1：base_url=.../compatible-mode/v1 + endpoint=/v1/chat/completions → /chat/completions
        # 例2：base_url=.../api/coding/v3   + endpoint=/v1/chat/completions → /chat/completions
        # 例3：base_url=.../v1              + endpoint=/v1/chat/completions → /chat/completions
        endpoint = config['endpoint']
        base_path = parsed.path.strip('/') if parsed.path else ''
        if base_path:
            full_prefix = '/' + base_path + '/'
            if endpoint.startswith(full_prefix):
                endpoint = endpoint[len(base_path) + 1:]
                if not endpoint.startswith('/'):
                    endpoint = '/' + endpoint
            else:
                # base_url 以 /vN 结尾，endpoint 以 /vN 开头 → 去掉 endpoint 的版本前缀
                import re
                base_suffix = base_path.rsplit('/', 1)[-1] if '/' in base_path else base_path
                ep_version_match = re.match(r'^/(v\d+)/', endpoint)
                if re.match(r'^v\d+$', base_suffix) and ep_version_match:
                    endpoint = endpoint[len(ep_version_match.group(1)) + 1:]
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
    def auto_detect_anthropic_url(cls, base_url: str) -> str:
        """根据已知厂商 OpenAI URL 推断对应的 Anthropic 兼容端点 URL。
        返回空字符串表示该厂商无 Anthropic 兼容端点或未知。
        """
        if not base_url:
            return ''
        url = base_url.rstrip('/')
        lower = url.lower()

        # 火山方舟 Doubao：OpenAI /api/v3 ↔ Anthropic /api/v3 (内部分流到 /v1/messages)
        # 实测两套接口共用一个 base，差异在请求路径
        if 'ark.cn-beijing.volces.com' in lower:
            # /api/v3/chat/completions ↔ /api/v3/anthropic/v1/messages
            # ForwardAnthropic 会拼接 /v1/messages，所以 anthropic_base 给到 .../api/v3/anthropic
            if '/api/v3/anthropic' in lower:
                return url
            if lower.endswith('/api/v3') or '/api/v3' in lower:
                return url.split('/api/v3')[0] + '/api/v3/anthropic'
            return url + '/anthropic'

        # 智谱 GLM：OpenAI /api/paas/v4 ↔ Anthropic /api/anthropic
        if 'open.bigmodel.cn' in lower:
            return 'https://open.bigmodel.cn/api/anthropic'

        # DashScope：OpenAI /compatible-mode/v1 ↔ Anthropic /api/v2/apps/anthropic
        if 'dashscope.aliyuncs.com' in lower:
            return 'https://dashscope.aliyuncs.com/api/v2/apps/anthropic'

        # Anthropic 官方
        if 'api.anthropic.com' in lower:
            return url  # 主 URL 就是 Anthropic 端点

        # OpenAI / Moonshot / DeepSeek / Mistral 等仅 OpenAI
        return ''

    # 已知 vendor 的 Anthropic 端点：(host, path_prefix)
    # path_prefix 按 "/" 分段匹配，避免 /api/coding 误命中 /api/coding-openai 等。
    # 空 path_prefix 表示整个 host 都是 Anthropic 协议。
    ANTHROPIC_URL_PATTERNS = (
        ('api.anthropic.com', ''),
        ('ark.cn-beijing.volces.com', '/api/v3/anthropic'),
        ('ark.cn-beijing.volces.com', '/api/coding'),
        ('open.bigmodel.cn', '/api/anthropic'),
        ('dashscope.aliyuncs.com', '/api/v2/apps/anthropic'),
    )

    OPENAI_URL_PATTERNS = (
        ('api.openai.com', ''),
        ('api.deepseek.com', ''),
        ('api.moonshot.cn', ''),
        ('api.mistral.ai', ''),
        ('api.groq.com', ''),
        ('api.x.ai', ''),
        ('generativelanguage.googleapis.com', ''),
        ('open.bigmodel.cn', '/api/paas'),
        ('dashscope.aliyuncs.com', '/compatible-mode'),
        ('ark.cn-beijing.volces.com', '/api/v3'),  # 注意需在 anthropic 子路径之后判断
    )

    @classmethod
    def _match_url_pattern(cls, base_url: str, patterns):
        """对 (host, path_prefix) 列表做 host 精确 + path 段前缀匹配。
        命中返回 "host+path"，未命中返回空字符串。
        """
        if not base_url:
            return ''
        from urllib.parse import urlparse
        try:
            parsed = urlparse(base_url.lower())
        except Exception:
            return ''
        host = (parsed.hostname or '').strip()
        # 把 path 拆成段，便于做精确前缀匹配（/api/coding 不会匹配 /api/coding-openai）
        path_segs = [s for s in (parsed.path or '').strip('/').split('/') if s]
        for p_host, p_path in patterns:
            if host != p_host:
                continue
            if not p_path:
                return p_host
            target_segs = [s for s in p_path.strip('/').split('/') if s]
            if len(path_segs) >= len(target_segs) and path_segs[:len(target_segs)] == target_segs:
                return p_host + p_path
        return ''

    @classmethod
    def auto_detect_api_format(cls, base_url: str):
        """识别 base_url 是 OpenAI 还是 Anthropic 协议端点。
        返回 (format, matched_pattern)：
          format ∈ {'anthropic', 'openai', 'auto'}
          matched_pattern 为命中的 host+path（auto 时为空字符串）
        """
        if not base_url:
            return 'auto', ''
        # Anthropic 子路径必须先判断（火山方舟 /api/v3/anthropic 在 /api/v3 之前）
        m = cls._match_url_pattern(base_url, cls.ANTHROPIC_URL_PATTERNS)
        if m:
            return 'anthropic', m
        m = cls._match_url_pattern(base_url, cls.OPENAI_URL_PATTERNS)
        if m:
            return 'openai', m
        return 'auto', ''

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
