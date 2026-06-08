# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""
知识库合规 v2.0 模拟测试
测试新增功能：
1. 审计日志表 (wr_audit_log) + API
2. AuditLog 模型
3. 知识捕获 + 截断 + 审计流程
4. 知识提取 + raw删除 + 审计流程
5. 数据保留清理
6. 前端合规说明 & 审计日志 tab

运行：cd /root/.openclaw/workspace/webrouter && python3 backend/test_knowledge_compliance.py
"""
import json
import sys
import os
import tempfile
import unittest
from datetime import datetime, timedelta

# 确保 backend 在 Python path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

# 设置测试数据库
TEST_DB = tempfile.mktemp(suffix='.db')
os.environ['DATABASE_URI'] = f'sqlite:///{TEST_DB}'

from app import create_app
from extensions import db
from models.knowledge import (
    KnowledgeRaw, KnowledgeItem, KnowledgeDomain,
    KnowledgeDomainRisk, KnowledgeAnalysis, AuditLog,
)


class TestConfig:
    """测试配置"""
    SECRET_KEY = 'test-secret-key'
    SQLALCHEMY_DATABASE_URI = 'sqlite://'  # in-memory, fresh per app
    SQLALCHEMY_TRACK_MODIFICATIONS = False
    DEBUG = True
    TESTING = True
    REDIS_URL = 'redis://localhost:6379/0'
    HEALTH_CHECK_INTERVAL = 300
    BALANCE_CHECK_INTERVAL = 1800
    ALERT_COOLDOWN = 300


class TestBase(unittest.TestCase):
    """测试基类：自动清理数据"""

    def setUp(self):
        self.app = create_app(TestConfig)
        self.client = self.app.test_client()
        # 清理所有表数据
        with self.app.app_context():
            for table in reversed(db.metadata.sorted_tables):
                db.session.execute(table.delete())
            db.session.commit()


class TestAuditLogModel(TestBase):
    """测试 AuditLog 数据模型"""

    def test_audit_log_model_exists(self):
        """验证 AuditLog 模型已注册"""
        self.assertEqual(AuditLog.__tablename__, 'wr_audit_log')
        # 验证字段
        cols = [c.name for c in AuditLog.__table__.columns]
        expected = ['id', 'action', 'resource_type', 'resource_id',
                    'token_id', 'detail', 'client_ip', 'created_at']
        for col in expected:
            self.assertIn(col, cols, f"Missing column: {col}")

    def test_audit_log_create_and_query(self):
        """创建并查询审计日志"""
        with self.app.app_context():
            entry = AuditLog(
                action='knowledge_capture',
                resource_type='raw',
                resource_id='req-abc-123',
                token_id=1,
                detail=json.dumps({"model": "gpt-4", "turns": 3}),
                client_ip='10.0.0.1',
            )
            db.session.add(entry)
            db.session.commit()

            # 查询验证
            found = AuditLog.query.filter_by(action='knowledge_capture').first()
            self.assertIsNotNone(found)
            self.assertEqual(found.resource_type, 'raw')
            self.assertEqual(found.resource_id, 'req-abc-123')
            self.assertEqual(found.token_id, 1)
            detail = json.loads(found.detail)
            self.assertEqual(detail['model'], 'gpt-4')
            self.assertEqual(found.client_ip, '10.0.0.1')

    def test_audit_log_to_dict(self):
        """测试 to_dict 方法"""
        entry = AuditLog(
            action='config_change',
            resource_type='config',
            resource_id='reload_all',
            token_id=0,
            detail='{}',
            client_ip='',
        )
        d = entry.to_dict()
        self.assertIn('id', d)
        self.assertIn('action', d)
        self.assertIn('created_at', d)
        self.assertEqual(d['action'], 'config_change')


class TestAuditLogAPI(TestBase):
    """测试审计日志 API 端点"""

    def _seed_audit_logs(self, count=5):
        with self.app.app_context():
            actions = ['knowledge_capture', 'knowledge_extract', 'config_change',
                       'raw_cleanup', 'retention_cleanup']
            for i in range(count):
                entry = AuditLog(
                    action=actions[i % len(actions)],
                    resource_type='raw' if i % 2 == 0 else 'item',
                    resource_id=f'req-{i}',
                    token_id=i + 1 if i > 0 else 0,
                    detail=json.dumps({"test": f"entry_{i}"}),
                    client_ip='10.0.0.1',
                )
                db.session.add(entry)
            db.session.commit()

    def test_audit_log_list_empty(self):
        """空列表返回"""
        with self.app.app_context():
            resp = self.client.get('/api/knowledge/audit_log')
            self.assertEqual(resp.status_code, 200)
            data = resp.get_json()
            self.assertEqual(data['total'], 0)
            self.assertEqual(data['items'], [])

    def test_audit_log_list_with_data(self):
        """有数据时返回分页结果"""
        self._seed_audit_logs(5)
        resp = self.client.get('/api/knowledge/audit_log?page=1&per_page=3')
        self.assertEqual(resp.status_code, 200)
        data = resp.get_json()
        self.assertEqual(data['total'], 5)
        self.assertEqual(len(data['items']), 3)
        self.assertEqual(data['page'], 1)
        self.assertEqual(data['per_page'], 3)

    def test_audit_log_filter_by_action(self):
        """按 action 筛选"""
        self._seed_audit_logs(5)
        resp = self.client.get('/api/knowledge/audit_log?action=knowledge_capture')
        self.assertEqual(resp.status_code, 200)
        data = resp.get_json()
        # 5条数据中 knowledge_capture 出现在 0, 3 索引
        self.assertGreater(data['total'], 0)
        for item in data['items']:
            self.assertEqual(item['action'], 'knowledge_capture')

    def test_audit_log_filter_by_resource_type(self):
        """按 resource_type 筛选"""
        self._seed_audit_logs(5)
        resp = self.client.get('/api/knowledge/audit_log?resource_type=raw')
        self.assertEqual(resp.status_code, 200)
        data = resp.get_json()
        self.assertGreater(data['total'], 0)
        for item in data['items']:
            self.assertEqual(item['resource_type'], 'raw')

    def test_audit_log_filter_by_token_id(self):
        """按 token_id 筛选"""
        self._seed_audit_logs(5)
        resp = self.client.get('/api/knowledge/audit_log?token_id=2')
        self.assertEqual(resp.status_code, 200)
        data = resp.get_json()
        self.assertGreater(data['total'], 0)
        for item in data['items']:
            self.assertEqual(item['token_id'], 2)

    def test_audit_log_pagination(self):
        """分页功能"""
        self._seed_audit_logs(10)
        # 第一页
        resp = self.client.get('/api/knowledge/audit_log?page=1&per_page=5')
        data1 = resp.get_json()
        self.assertEqual(len(data1['items']), 5)

        # 第二页
        resp = self.client.get('/api/knowledge/audit_log?page=2&per_page=5')
        data2 = resp.get_json()
        self.assertEqual(len(data2['items']), 5)

        # 验证两页数据不重叠
        ids1 = {i['id'] for i in data1['items']}
        ids2 = {i['id'] for i in data2['items']}
        self.assertEqual(ids1 & ids2, set())

    def test_audit_log_order_desc(self):
        """按 ID 降序排列（最新在前）"""
        self._seed_audit_logs(3)
        resp = self.client.get('/api/knowledge/audit_log?page=1&per_page=10')
        data = resp.get_json()
        items = data['items']
        for i in range(len(items) - 1):
            self.assertGreater(items[i]['id'], items[i + 1]['id'],
                               "Audit logs should be in descending order")


class TestKnowledgeRawTruncation(unittest.TestCase):
    """模拟测试 raw 表文本截断行为"""

    def test_raw_text_max_len_constant(self):
        """验证 Go 端的 rawTextMaxLen 常量应为 5000"""
        # 这是文档级验证 — 在 knowledge_db.go 中
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'knowledge_db.go'), 'r') as f:
            content = f.read()
        self.assertIn('const rawTextMaxLen = 5000', content)

    def test_saveKnowledgeRaw_uses_truncate(self):
        """验证 saveKnowledgeRaw 函数使用了 truncate"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'knowledge_db.go'), 'r') as f:
            content = f.read()
        # 确认 prompt 和 response 都被 truncate 处理
        self.assertIn('truncate(entry.Prompt, rawTextMaxLen)', content)
        self.assertIn('truncate(entry.Response, rawTextMaxLen)', content)

    def test_truncate_function_exists(self):
        """验证 truncate 函数在 mcp_tools.go 中存在"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'mcp_tools.go'), 'r') as f:
            content = f.read()
        self.assertIn('func truncate(s string, maxLen int) string', content)


class TestKnowledgeExtractionRawDeletion(unittest.TestCase):
    """模拟测试知识提取后 raw 原文删除行为"""

    def test_extract_deletes_raw_after_done(self):
        """验证 ExtractRawToKnowledge 在标记 done 后删除 raw 记录"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'knowledge_extract.go'), 'r') as f:
            content = f.read()
        # 确认在标记 done 后有 DELETE 操作
        self.assertIn("DELETE FROM wr_knowledge_raw WHERE id = ?", content)

    def test_delete_happens_after_done_marker(self):
        """验证 DELETE 紧跟在 done 标记之后"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'knowledge_extract.go'), 'r') as f:
            content = f.read()
        # 确认顺序：先 status='done'，然后 DELETE
        done_idx = content.find("status = 'done'")
        delete_idx = content.find("DELETE FROM wr_knowledge_raw")
        self.assertGreater(done_idx, -1, "status='done' marker not found")
        self.assertGreater(delete_idx, -1, "DELETE statement not found")
        self.assertGreater(delete_idx, done_idx, "DELETE should come after status='done'")


class TestAuditHooks(unittest.TestCase):
    """模拟测试各审计日志 Hook 接入点"""

    def test_capture_audit_hook(self):
        """验证知识捕获路径有审计日志"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'knowledge_capture.go'), 'r') as f:
            content = f.read()
        self.assertIn('LogAudit(AuditKnowledgeCapture', content)

    def test_extract_audit_hook(self):
        """验证知识提取路径有审计日志"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'knowledge_extract.go'), 'r') as f:
            content = f.read()
        self.assertIn('LogAudit(AuditKnowledgeExtract', content)

    def test_reload_audit_hook(self):
        """验证配置重载有审计日志"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'handlers.go'), 'r') as f:
            content = f.read()
        self.assertIn('LogConfigChange("reload_all"', content)

    def test_cleanup_audit_hook(self):
        """验证定时清理有审计日志"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'knowledge_stats.go'), 'r') as f:
            content = f.read()
        self.assertIn('LogConfigChange("raw_cleanup_30d"', content)
        self.assertIn('LogConfigChange("retention_cleanup"', content)

    def test_extract_api_audit_hook(self):
        """验证手动触发提取有审计日志"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'knowledge_stats.go'), 'r') as f:
            content = f.read()
        self.assertIn('LogConfigChange("knowledge_extract"', content)


class TestRetentionCleanup(unittest.TestCase):
    """测试数据保留期限 enforcement"""

    def test_cleanupExpiredKnowledge_function_exists(self):
        """验证 cleanupExpiredKnowledge 函数已实现"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'knowledge_db.go'), 'r') as f:
            content = f.read()
        self.assertIn('func cleanupExpiredKnowledge()', content)

    def test_retention_cleanup_scheduler_exists(self):
        """验证 startRetentionCleanup 调度器已实现"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'knowledge_stats.go'), 'r') as f:
            content = f.read()
        self.assertIn('func startRetentionCleanup()', content)

    def test_retention_scheduler_registered_in_main(self):
        """验证调度器在 main.go 中注册"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'main.go'), 'r') as f:
            content = f.read()
        self.assertIn('go startRetentionCleanup()', content)

    def test_cleanup_deletes_expired_items(self):
        """验证清理函数删除过期条目"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'knowledge_db.go'), 'r') as f:
            content = f.read()
        self.assertIn('retention_until IS NOT NULL', content)
        self.assertIn('retention_until < datetime', content)

    def test_cleanup_also_deletes_orphan_vectors(self):
        """验证清理函数同时删除孤立向量数据"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'knowledge_db.go'), 'r') as f:
            content = f.read()
        self.assertIn('DELETE FROM wr_knowledge_vectors', content)
        self.assertIn('item_id NOT IN (SELECT id FROM wr_knowledge_items)', content)

    def test_retention_audit_logged(self):
        """验证保留清理有审计日志"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'knowledge_stats.go'), 'r') as f:
            content = f.read()
        self.assertIn('LogConfigChange("retention_cleanup"', content)


class TestAuditLoggerInitialization(unittest.TestCase):
    """测试审计日志初始化"""

    def test_init_audit_logger_called(self):
        """验证 InitAuditLogger 在 main.go 中调用"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'main.go'), 'r') as f:
            content = f.read()
        self.assertIn('InitAuditLogger()', content)

    def test_audit_log_table_ddl(self):
        """验证 wr_audit_log 表 DDL"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'knowledge_db.go'), 'r') as f:
            content = f.read()
        self.assertIn('CREATE TABLE IF NOT EXISTS wr_audit_log', content)
        self.assertIn('action TEXT NOT NULL', content)
        self.assertIn('resource_type', content)
        self.assertIn('resource_id', content)
        self.assertIn('token_id', content)
        self.assertIn('detail', content)
        self.assertIn('client_ip', content)
        self.assertIn('created_at', content)

    def test_audit_log_indexes(self):
        """验证审计日志索引"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'wr-proxy', 'knowledge_db.go'), 'r') as f:
            content = f.read()
        self.assertIn('idx_audit_action', content)
        self.assertIn('idx_audit_created', content)
        self.assertIn('idx_audit_resource', content)


class TestAuditLogModelImportedInApp(unittest.TestCase):
    """验证 AuditLog 在 app.py 中正确导入"""

    def test_audit_log_in_app_imports(self):
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               'app.py'), 'r') as f:
            content = f.read()
        self.assertIn('AuditLog', content)

    def test_audit_log_in_knowledge_routes_imports(self):
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               'routes', 'knowledge_routes.py'), 'r') as f:
            content = f.read()
        self.assertIn('AuditLog', content)


class TestFrontendComplianceTab(unittest.TestCase):
    """测试前端合规说明 & 审计日志 tab"""

    def test_compliance_tab_registered(self):
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'backend', 'static', 'js', 'knowledge.js'), 'r') as f:
            content = f.read()
        self.assertIn("switchTab('compliance')", content)
        self.assertIn("renderCompliance", content)

    def test_audit_tab_registered(self):
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'backend', 'static', 'js', 'knowledge.js'), 'r') as f:
            content = f.read()
        self.assertIn("switchTab('audit')", content)
        self.assertIn("loadAuditLog", content)

    def test_compliance_content_has_internal_notice(self):
        """验证合规说明包含内部告知内容"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'backend', 'static', 'js', 'knowledge.js'), 'r') as f:
            content = f.read()
        self.assertIn('内部告知', content)
        self.assertIn('自动捕获对话内容', content)
        self.assertIn('脱敏处理', content)
        self.assertIn('长度截断', content)

    def test_compliance_content_has_purpose_limitation(self):
        """验证合规说明包含目的限制声明"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'backend', 'static', 'js', 'knowledge.js'), 'r') as f:
            content = f.read()
        self.assertIn('目的限制声明', content)
        self.assertIn('不得', content)
        self.assertIn('绩效', content)
        self.assertIn('监控', content)

    def test_audit_frontend_calls_correct_api(self):
        """验证前端审计日志调用正确的 API"""
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                               '..', 'backend', 'static', 'js', 'knowledge.js'), 'r') as f:
            content = f.read()
        self.assertIn('/api/knowledge/audit_log', content)


class TestFullKnowledgeFlowSimulation(TestBase):
    """全流程模拟测试：创建 raw -> 查询 -> 审计日志 API"""

    def setUp(self):
        self.app = create_app(TestConfig)
        self.client = self.app.test_client()

    def test_raw_create_and_list(self):
        """模拟 raw 数据创建和列表查询"""
        with self.app.app_context():
            raw = KnowledgeRaw(
                request_id='sim-test-001',
                token_id=1,
                token_name='test-token',
                model_name='gpt-4',
                prompt='分析Q3财报',
                response='Q3毛利率同比收窄3.2个百分点至18.7%',
                turn_count=3,
                client_ip='10.0.0.1',
                status='pending',
            )
            db.session.add(raw)
            db.session.commit()

            # 通过 API 验证列表
            resp = self.client.get('/api/knowledge/raw?page=1&per_page=10')
            self.assertEqual(resp.status_code, 200)
            data = resp.get_json()
            self.assertGreater(data['total'], 0)

            # 验证详情
            resp = self.client.get(f'/api/knowledge/raw/{raw.id}')
            self.assertEqual(resp.status_code, 200)
            d = resp.get_json()
            self.assertEqual(d['request_id'], 'sim-test-001')
            self.assertEqual(d['model_name'], 'gpt-4')

    def test_audit_log_full_flow(self):
        """模拟审计日志完整流程：创建 -> 查询 -> 筛选"""
        with self.app.app_context():
            # 创建多条审计日志
            for i in range(3):
                entry = AuditLog(
                    action='knowledge_capture' if i < 2 else 'knowledge_extract',
                    resource_type='raw',
                    resource_id=f'sim-req-{i}',
                    token_id=1,
                    detail=json.dumps({"model": "gpt-4", "turn": i + 1}),
                    client_ip='10.0.0.1',
                )
                db.session.add(entry)
            db.session.commit()

            # 列表查询
            resp = self.client.get('/api/knowledge/audit_log?page=1&per_page=10')
            self.assertEqual(resp.status_code, 200)
            data = resp.get_json()
            self.assertEqual(data['total'], 3)

            # 筛选 knowledge_capture
            resp = self.client.get('/api/knowledge/audit_log?action=knowledge_capture')
            data = resp.get_json()
            self.assertEqual(data['total'], 2)

            # 筛选 knowledge_extract
            resp = self.client.get('/api/knowledge/audit_log?action=knowledge_extract')
            data = resp.get_json()
            self.assertEqual(data['total'], 1)


if __name__ == '__main__':
    print("=" * 60)
    print("知识库合规 v2.0 模拟测试")
    print("=" * 60)
    unittest.main(verbosity=2)
