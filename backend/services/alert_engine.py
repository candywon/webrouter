# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""告警引擎 — 评估告警规则并发送通知"""
import json, time, logging, asyncio, smtplib
from email.mime.text import MIMEText
from datetime import datetime
import aiohttp
from extensions import db
from models.wr_models import AlertRule, AlertHistory

logger = logging.getLogger(__name__)


class WeChatAlertChannel:
    """Server酱微信推送"""
    def __init__(self, sendkey: str = ''):
        self.sendkey = sendkey
        self.url = f"https://sctapi.ftqq.com/{sendkey}.send" if sendkey else None

    async def send(self, title: str, content: str):
        if not self.url:
            logger.warning("微信告警未配置 sendkey，跳过发送")
            return
        async with aiohttp.ClientSession() as s:
            resp = await s.post(self.url, data={"title": title, "desp": content}, timeout=aiohttp.ClientTimeout(total=10))
            if resp.status != 200:
                logger.warning(f"Server酱推送失败: HTTP {resp.status}")


class EmailAlertChannel:
    """SMTP 邮件告警（使用 Python 内置 smtplib）"""
    def __init__(self, to_addr: str = '', smtp_host: str = '', smtp_port: int = 587,
                 smtp_user: str = '', smtp_password: str = '', smtp_use_tls: bool = True,
                 from_addr: str = ''):
        self.to_addr = to_addr
        self.smtp_host = smtp_host
        self.smtp_port = smtp_port
        self.smtp_user = smtp_user
        self.smtp_password = smtp_password
        self.smtp_use_tls = smtp_use_tls
        self.from_addr = from_addr or smtp_user

    async def send(self, title: str, content: str):
        if not self.smtp_host or not self.to_addr:
            logger.warning("邮件告警未配置 SMTP 或收件人，跳过发送")
            return
        loop = asyncio.get_event_loop()
        await loop.run_in_executor(None, self._sync_send, title, content)

    def _sync_send(self, title, content):
        msg = MIMEText(content, 'plain', 'utf-8')
        msg['Subject'] = f'[WebRouter Alert] {title}'
        msg['From'] = self.from_addr
        msg['To'] = self.to_addr

        try:
            if self.smtp_port == 465:
                server = smtplib.SMTP_SSL(self.smtp_host, self.smtp_port, timeout=15)
            else:
                server = smtplib.SMTP(self.smtp_host, self.smtp_port, timeout=15)
                server.ehlo()
                server.starttls()

            if self.smtp_user and self.smtp_password:
                server.login(self.smtp_user, self.smtp_password)

            server.sendmail(self.from_addr, self.to_addr.split(','), msg.as_string())
            server.quit()
            logger.info(f"邮件告警已发送: to={self.to_addr}")
        except Exception as e:
            logger.error(f"邮件告警发送失败: {e}")


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
        channels = []
        for n in names:
            cls = CHANNEL_MAP.get(n)
            if cls:
                channels.append(cls(**cfg.get(n, {})))
        return channels

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
