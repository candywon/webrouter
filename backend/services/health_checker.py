"""统一健康检测服务 — 支持所有 Provider 类型"""
import time
import logging
from datetime import datetime
from extensions import db
from models.provider import Provider
from models.wr_models import ChannelHealth

logger = logging.getLogger(__name__)


class HealthChecker:
    """统一健康检测引擎 — 检测所有已注册的 Provider"""

    def check_provider(self, provider_dict: dict) -> dict:
        """检测单个 Provider — 根据类型自动选择检测策略"""
        from models.provider_factory import ProviderFactory
        adapter = ProviderFactory.create(provider_dict)
        return adapter.check_health()

    def check_all_providers(self) -> list:
        """检测所有已启用的 Provider"""
        providers = Provider.query.filter_by(enabled=True).all()
        results = []

        for p in providers:
            try:
                result = self.check_provider(p.to_dict(include_secrets=True))

                # 更新 Provider 状态
                p.status = result.get('status', 'unknown')
                p.last_check_at = datetime.utcnow()
                p.last_latency_ms = result.get('latency_ms')
                p.last_error = result.get('error')
                p.updated_at = datetime.utcnow()

                # 写入健康历史
                health = ChannelHealth(
                    provider_id=p.id,
                    status=result.get('status', 'unknown'),
                    latency_ms=result.get('latency_ms'),
                    error_message=result.get('error'),
                )
                db.session.add(health)

                result['provider_id'] = p.id
                result['name'] = p.name
                result['type'] = p.type
                results.append(result)

            except Exception as e:
                logger.error(f"Provider {p.name} (id={p.id}) health check failed: {e}")
                p.status = 'dead'
                p.last_error = str(e)[:200]
                p.last_check_at = datetime.utcnow()
                results.append({
                    'provider_id': p.id,
                    'name': p.name,
                    'type': p.type,
                    'status': 'dead',
                    'latency_ms': 0,
                    'error': str(e)[:200],
                })

        db.session.commit()
        return results
