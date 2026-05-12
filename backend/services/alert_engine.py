"""告警引擎 — 评估告警规则并发送通知"""
import json, time, logging, asyncio
from datetime import datetime
import aiohttp
from extensions import db
from models.wr_models import AlertRule, AlertHistory

logger = logging.getLogger(__name__)


class WeChatAlertChannel:
    """Server酱微信推送"""
    def __init__(self, sendkey: str):
        self.url = f"https://sctapi.ftqq.com/{sendkey}.send"

    async def send(self, title: str, content: str):
        async with aiohttp.ClientSession() as s:
            await s.post(self.url, data={"title": title, "desp": content})


class EmailAlertChannel:
    """邮件告警渠道"""
    def __init__(self, to_addr: str, mail=None):
        self.to_addr = to_addr
        self.mail = mail

    async def send(self, title: str, content: str):
        if not self.mail:
            return
        loop = asyncio.get_event_loop()
        await loop.run_in_executor(None, self._sync_send, title, content)

    def _sync_send(self, title, content):
        from flask import current_app
        with current_app.app_context():
            self.mail.send_message(subject=f"[WebRouter告警] {title}",
                                   recipients=[self.to_addr], body=content)


class WebhookAlertChannel:
    """通用Webhook推送"""
    def __init__(self, url: str, headers: dict = None):
        self.url = url
        self.headers = headers or {}

    async def send(self, title: str, content: str):
        async with aiohttp.ClientSession() as s:
            await s.post(self.url, json={"title": title, "content": content},
                         headers=self.headers)


CHANNEL_MAP = {"wechat": WeChatAlertChannel, "email": EmailAlertChannel, "webhook": WebhookAlertChannel}


def _evaluate(cond_type: str, config: dict, event: dict) -> bool:
    """评估单条规则条件是否触发"""
    if cond_type == "key_failed":
        return event.get("key_id") == config.get("key_id") and event.get("status") == "failed"
    if cond_type == "balance_low":
        return float(event.get("balance", 0)) < float(config.get("threshold", 0))
    if cond_type == "error_rate":
        return float(event.get("error_rate", 0)) > float(config.get("threshold", 0))
    if cond_type == "usage_spike":
        baseline = float(event.get("baseline", 0)) or 1
        return float(event.get("current", 0)) > baseline * float(config.get("multiplier", 2))
    return False


class AlertEngine:
    """告警引擎：评估规则 → 冷却检查 → 异步通知 → 写入历史"""

    def __init__(self, app=None, cooldown_seconds: int = 300):
        self.app = app
        self.cooldown_seconds = cooldown_seconds
        self._cooldown: dict[int, float] = {}  # rule_id → 上次触发时间戳

    def _is_cooled(self, rule_id: int) -> bool:
        """冷却期内不重复触发"""
        return time.time() - self._cooldown.get(rule_id, 0) >= self.cooldown_seconds

    def _mark_fired(self, rule_id: int):
        self._cooldown[rule_id] = time.time()

    def _build_channels(self, names: list[str], cfg: dict):
        return [CHANNEL_MAP[n](**cfg.get(n, {})) for n in names if n in CHANNEL_MAP]

    def evaluate_event(self, event: dict, channel_config: dict | None = None):
        """评估事件，返回触发规则ID列表"""
        channel_config = channel_config or {}
        rules = AlertRule.query.filter_by(enabled=True).all()
        fired = []
        for rule in rules:
            if not self._is_cooled(rule.id):
                continue
            config = json.loads(rule.condition_config) if isinstance(rule.condition_config, str) else rule.condition_config
            if not _evaluate(rule.condition_type, config, event):
                continue
            self._mark_fired(rule.id)
            ch_names = json.loads(rule.channels) if isinstance(rule.channels, str) else rule.channels
            channels = self._build_channels(ch_names, channel_config)
            msg = f"[{rule.level.upper()}] {rule.name}: {json.dumps(event, ensure_ascii=False)}"
            self._fire(rule, event, msg, channels)
            fired.append(rule.id)
        return fired

    def _fire(self, rule, event, message, channels):
        """异步发送通知 + 写入历史"""
        ch_names = [c.__class__.__name__ for c in channels]
        try:
            loop = asyncio.get_event_loop()
            if loop.is_running():
                for ch in channels:
                    asyncio.ensure_future(ch.send(rule.name, message))
            else:
                loop.run_until_complete(self._send_all(channels, rule.name, message))
        except RuntimeError:
            asyncio.run(self._send_all(channels, rule.name, message))

        db.session.add(AlertHistory(
            rule_id=rule.id, event_data=json.dumps(event, ensure_ascii=False),
            message=message, level=rule.level,
            channels_sent=json.dumps(ch_names),
        ))
        db.session.commit()
        logger.info(f"告警触发: rule={rule.name}, channels={ch_names}")

    @staticmethod
    async def _send_all(channels, title, content):
        results = await asyncio.gather(*[c.send(title, content) for c in channels], return_exceptions=True)
        for r in results:
            if isinstance(r, Exception):
                logger.warning(f"告警发送失败: {r}")
