"""New-API数据库适配器 — 只读访问New-API表"""
import logging
from extensions import db
from sqlalchemy import text

logger = logging.getLogger(__name__)


class NewAPIAdapter:
    """直读New-API数据库，获取渠道/用户/日志数据"""

    @staticmethod
    def get_channels():
        """获取所有渠道列表及状态"""
        sql = text("""
            SELECT id, name, type, status, priority, weight,
                   models, base_url, other, created_time, test_time
            FROM channels
            ORDER BY priority DESC, id ASC
        """)
        result = db.session.execute(sql)
        return [dict(row._mapping) for row in result]

    @staticmethod
    def get_channel_by_id(channel_id):
        """获取单个渠道详情"""
        sql = text("""
            SELECT id, name, type, status, priority, weight,
                   models, base_url, other, created_time, test_time
            FROM channels WHERE id = :cid
        """)
        result = db.session.execute(sql, {'cid': channel_id}).fetchone()
        return dict(result._mapping) if result else None

    @staticmethod
    def get_tokens():
        """获取所有API Token"""
        sql = text("""
            SELECT id, name, key, status, used_quota, remain_quota,
                   unlimited_quota, models, created_time
            FROM tokens
            ORDER BY id ASC
        """)
        result = db.session.execute(sql)
        return [dict(row._mapping) for row in result]

    @staticmethod
    def get_users():
        """获取所有用户"""
        sql = text("""
            SELECT id, username, display_name, email, role, status,
                   quota, used_quota, created_time
            FROM users
            ORDER BY id ASC
        """)
        result = db.session.execute(sql)
        return [dict(row._mapping) for row in result]

    @staticmethod
    def get_usage_stats(hours=24):
        """获取用量统计"""
        sql = text(f"""
            SELECT model_name, channel_id,
                   COUNT(*) as request_count,
                   COALESCE(SUM(prompt_tokens), 0) as input_tokens,
                   COALESCE(SUM(completion_tokens), 0) as output_tokens,
                   AVG(duration) as avg_duration,
                   SUM(CASE WHEN code != 200 THEN 1 ELSE 0 END) as error_count
            FROM logs
            WHERE created_at >= datetime('now', '-{hours} hours')
            GROUP BY model_name, channel_id
            ORDER BY request_count DESC
        """)
        try:
            result = db.session.execute(sql)
            return [dict(row._mapping) for row in result]
        except Exception as e:
            logger.warning(f"Failed to query usage stats: {e}")
            return []

    @staticmethod
    def get_daily_usage(days=7):
        """获取每日用量趋势"""
        sql = text(f"""
            SELECT DATE(created_at) as date,
                   COUNT(*) as request_count,
                   COALESCE(SUM(prompt_tokens), 0) as input_tokens,
                   COALESCE(SUM(completion_tokens), 0) as output_tokens,
                   SUM(CASE WHEN code != 200 THEN 1 ELSE 0 END) as error_count
            FROM logs
            WHERE created_at >= datetime('now', '-{days} days')
            GROUP BY DATE(created_at)
            ORDER BY date ASC
        """)
        try:
            result = db.session.execute(sql)
            return [dict(row._mapping) for row in result]
        except Exception as e:
            logger.warning(f"Failed to query daily usage: {e}")
            return []

    @staticmethod
    def get_error_logs(limit=50):
        """获取最近错误日志"""
        sql = text("""
            SELECT id, user_id, channel_id, model_name,
                   prompt_tokens, completion_tokens, code,
                   duration, created_at
            FROM logs
            WHERE code != 200
            ORDER BY created_at DESC
            LIMIT :limit
        """)
        try:
            result = db.session.execute(sql, {'limit': limit})
            return [dict(row._mapping) for row in result]
        except Exception as e:
            logger.warning(f"Failed to query error logs: {e}")
            return []
