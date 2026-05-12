"""统计采集器 — 定期从New-API数据库采集用量统计"""
import json, logging
from datetime import datetime
from apscheduler.schedulers.background import BackgroundScheduler
from extensions import db, redis_client
from models.wr_models import CostRecord
from models.newapi_adapter import NewAPIAdapter

logger = logging.getLogger(__name__)

STATS_KEY_PREFIX = "wr:stats"
STATS_TTL_SECONDS = 1800  # 5分钟聚合保留30分钟


class StatsCollector:
    """统计采集器：定时从New-API日志聚合用量，写Redis+DB"""

    def __init__(self, app=None, interval_minutes: int = 5):
        self.app = app
        self.interval_minutes = interval_minutes
        self._scheduler = BackgroundScheduler()

    def start(self):
        """启动定时采集"""
        self._scheduler.add_job(self._run, "interval", minutes=self.interval_minutes,
                                id="stats_collection", replace_existing=True)
        self._scheduler.start()
        logger.info(f"统计采集器已启动，间隔{self.interval_minutes}分钟")

    def shutdown(self):
        self._scheduler.shutdown(wait=False)

    def _run(self):
        if self.app:
            with self.app.app_context():
                self._collect()
        else:
            self._collect()

    def _collect(self):
        now = datetime.utcnow()
        period_key = now.strftime("%Y%m%d%H%M")
        raw = NewAPIAdapter.get_usage_stats(hours=1)
        if not raw:
            logger.debug("无新用量数据，跳过")
            return

        # 聚合写入Redis
        by_model = self._aggregate(raw, key="model_name", out_key="model")
        by_channel = self._aggregate(raw, key="channel_id", out_key="channel_id")
        self._to_redis("model", period_key, by_model)
        self._to_redis("channel", period_key, by_channel)

        # 持久化成本记录
        self._persist(raw, now)
        logger.info(f"统计采集完成: {len(raw)}条, period={period_key}")

    @staticmethod
    def _aggregate(rows: list[dict], key: str, out_key: str) -> list[dict]:
        """按指定维度聚合：请求数/Token/错误率/平均延迟"""
        buckets: dict = {}
        for r in rows:
            k = r.get(key, "unknown")
            b = buckets.setdefault(k, {"rc": 0, "inp": 0, "out": 0,
                                       "err": 0, "dur": 0.0})
            rc = r.get("request_count", 0)
            b["rc"] += rc
            b["inp"] += r.get("input_tokens", 0) or 0
            b["out"] += r.get("output_tokens", 0) or 0
            b["err"] += r.get("error_count", 0) or 0
            b["dur"] += (r.get("avg_duration", 0) or 0) * max(rc, 1)

        result = []
        for k, b in buckets.items():
            rc = max(b["rc"], 1)
            result.append({
                out_key: k, "request_count": b["rc"],
                "input_tokens": b["inp"], "output_tokens": b["out"],
                "error_rate": round(b["err"] / rc, 4),
                "avg_latency_ms": round(b["dur"] / rc, 1),
            })
        return result

    def _to_redis(self, stats_type: str, period: str, data: list):
        """写入Redis wr:stats:{type}:{period}"""
        k = f"{STATS_KEY_PREFIX}:{stats_type}:{period}"
        try:
            redis_client.setex(k, STATS_TTL_SECONDS, json.dumps(data, ensure_ascii=False))
        except Exception as e:
            logger.warning(f"Redis写入失败 key={k}: {e}")

    def _persist(self, rows: list[dict], now: datetime):
        """写入wr_cost_records持久化成本记录"""
        try:
            for r in rows:
                inp = r.get("input_tokens", 0) or 0
                out = r.get("output_tokens", 0) or 0
                # 简易估算: input $3/M tokens, output $15/M tokens (单位:分)
                cost_cents = int(inp * 0.003 + out * 0.015)
                if inp == 0 and out == 0 and cost_cents <= 0:
                    continue
                db.session.add(CostRecord(
                    channel_id=r.get("channel_id"), model_name=r.get("model_name", "unknown"),
                    input_tokens=inp, output_tokens=out,
                    cost_cents=max(cost_cents, 0), recorded_at=now,
                ))
            db.session.commit()
        except Exception as e:
            db.session.rollback()
            logger.warning(f"成本记录写入失败: {e}")
