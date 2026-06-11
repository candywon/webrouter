# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""WebRouter backend i18n — lightweight message translation, no external deps."""

MESSAGES = {
    'en': {
        # Auth
        'login_success': 'Login successful',
        'logout_success': 'Logged out',
        'invalid_credentials': 'Invalid username or password',
        'username_password_required': 'Username and password are required',
        'wrong_password': 'Incorrect password',
        'password_changed': 'Password changed',
        'new_password_min_length': 'New password must be at least 8 characters',

        # Generic CRUD
        'created': 'Created successfully',
        'updated': 'Updated successfully',
        'deleted': 'Deleted successfully',
        'create_failed': 'Create failed',
        'update_failed': 'Update failed',
        'delete_failed': 'Delete failed',
        'not_found': 'Not found',
        'already_exists': 'Already exists',
        'no_data': 'No data',

        # Validation
        'name_required': 'Name is required',
        'url_required': 'URL is required',
        'key_required': 'Key is required',
        'model_required': 'Model is required',
        'password_required': 'Password is required',
        'pattern_required': 'Pattern is required',
        'channel_name_required': 'Channel name is required',
        'rule_name_required': 'Rule name is required',
        'domain_code_required': 'domain_code is required',
        'domain_name_required': 'domain_code and domain_name are required',
        'invalid_format_dept_name_email': 'Invalid format, expected: Department Name email',
        'invalid_expires_at': 'expires_at format is invalid',
        'invalid_org_type': 'org_type must be company/department/group',
        'invalid_org_type_value': 'org_type is invalid',
        'invalid_type': 'type must be {VALID_TYPES}',
        'invalid_level': 'level must be {VALID_LEVELS}',
        'invalid_tier': 'tier must be one of {VALID_TIERS}',
        'invalid_category': 'category must be {VALID_CATEGORIES}',
        'invalid_desensitize_level': 'desensitize_level must be off/standard/strict',
        'invalid_regex': 'Regex syntax error: {e}',
        'unsupported_type': 'Unsupported type: {provider_type}',

        # Field validation
        'field_required_model': 'model is required',
        'field_required_alias_target': 'alias and target are required',
        'field_required_target': 'target is required',
        'field_required_target_code': 'target_code is required',
        'field_required_ids': 'ids is required',
        'field_required_token_id': 'token_id is required',
        'field_required_text': 'text field is required',
        'field_required_channels_array': 'channels array is required',
        'field_required_items_array': 'items array is required',

        # Provider
        'provider_created': 'Provider created',
        'provider_updated': 'Provider updated',
        'provider_deleted': 'Provider deleted',
        'provider_not_found': 'Provider not found',
        'base_url_required': 'Base URL is required',

        # Channel
        'channel_created': 'Channel created',
        'channel_updated': 'Channel updated',
        'channel_batch_created': 'Batch created {len} channels',
        'channel_batch_updated': 'Batch updated: {created} created, {updated} updated',

        # Token / Member
        'token_created': 'Token created',
        'token_updated': 'Token updated',
        'token_deleted': 'Token deleted',
        'token_not_found': 'Token not found',
        'member_created': 'Member created',
        'member_updated': 'Member updated',
        'member_not_found': 'Member not found',
        'member_transferred': 'Member transferred',

        # Org
        'org_created': 'Organization created',
        'org_updated': 'Organization updated',
        'org_deleted': 'Organization deleted',
        'org_not_found': 'Organization not found',
        'org_name_required': 'Organization name is required',
        'org_id_not_found': 'Organization ID {org_id} not found',
        'parent_org_not_found': 'Parent organization not found',
        'target_org_not_found': 'Target organization not found',
        'cannot_set_self_as_parent': 'Cannot set self as parent',
        'org_has_children': 'This organization has sub-organizations. Delete them first.',
        'org_has_members': 'This organization still has members. Remove them first.',

        # Batch import
        'batch_import_done': 'Batch import completed: {success} succeeded, {failed} failed',
        'members_array_or_text_required': 'members array or text is required',

        # Settings
        'settings_updated': 'Settings updated',
        'settings_updated_count': 'Updated {len} settings',
        'setting_not_found': 'Setting {key} not found',
        'setting_not_editable': '{key}: this setting is not editable',
        'settings_initialized': 'Initialized {len} items',
        'db_commit_failed': 'Database commit failed: {e}',

        # Backup
        'backup_sqlite_only': 'Backup only supports SQLite',
        'restore_sqlite_only': 'Restore only supports SQLite',
        'backup_file_not_found': 'Backup file not found',
        'restored': 'Restored',

        # SMTP
        'smtp_no_server': 'SMTP server not configured',
        'smtp_no_recipient': 'Recipient address not configured',
        'smtp_no_credentials': 'SMTP username or password not configured',
        'smtp_test_sent': 'Test email sent to {email_to}',
        'smtp_auth_failed': 'SMTP authentication failed: {e}\nTip: For QQ Mail use an authorization code, not your login password.',
        'smtp_connection_failed': 'SMTP connection failed: {e}',
        'smtp_sender_rejected': 'Sender address rejected: {e}',
        'smtp_recipient_rejected': 'Recipient address rejected: {e}',
        'smtp_connection_closed': 'SMTP connection closed unexpectedly, possibly authentication failure or server rejection.\nTip: For QQ Mail, get an authorization code from Settings → Account → POP3/IMAP/SMTP, not your login password.',
        'smtp_send_failed': 'Email send failed: {e}',
        'network_error': 'Network error: {err_msg}',
        'unknown_error': 'Unknown error: {e}',

        # Proxy
        'proxy_reloaded': 'wr-proxy reloaded',
        'proxy_reload_error': 'wr-proxy returned {status}: {error}',
        'proxy_unreachable': 'Cannot reach wr-proxy ({proxy_url}). Verify wr-proxy is running.',
        'proxy_reload_failed': 'Reload failed: {e}',
        'request_failed': 'Request failed',
        'request_failed_detail': 'Request failed: {e}',
        'demo_test_response': 'This is a simulated response from WebRouter Demo. The API test feature works correctly — in production, this will forward requests to the real upstream AI provider.',
        'no_available_provider': 'No available provider for model "{model}". Possible causes: 1) Provider status is auth_failed/unhealthy (will auto-retry after backoff); 2) Model not configured in any enabled Provider; 3) All Providers are in cooldown. You can reload wr-proxy from Settings to force a refresh.',

        # Refresh / Cache
        'refresh_sent': 'Refresh request sent',
        'cooldown_clear_sent': 'Cooldown clear request sent',

        # Pricing
        'pricing_created': 'Pricing created',
        'pricing_updated': 'Pricing updated',
        'pricing_already_exists': '{model} already exists, use PUT to update',
        'cannot_delete_default_pricing': 'Cannot delete default pricing',

        # Model grades
        'model_grade_created': 'Model grade created',
        'model_grade_updated': 'Model grade updated',

        # Model aliases
        'model_alias_created': 'Model alias created',
        'model_alias_updated': 'Model alias updated',

        # Desensitization
        'desensitize_rule_created': 'Desensitization rule created',
        'desensitize_rule_updated': 'Desensitization rule updated',
        'desensitize_rule_deleted': 'Desensitization rule deleted',

        # Quota
        'quota_updated': 'Quota updated',
        'quota_reset': 'Quota reset',

        # Knowledge
        'knowledge_activated': 'Knowledge base activated',
        'knowledge_confirm_activation': 'Please confirm knowledge base activation',
        'domain_created': 'Domain created',
        'domain_updated': 'Domain updated',
        'domain_confirmed': 'Domain confirmed',
        'domain_code_exists': 'Domain code {code} already exists',
        'target_domain_not_found': 'Target domain {target_code} not found',
        'merged_to_domain': 'Merged into {domain}, migrated {migrated} knowledge items',
        'memory_updated': 'Memory updated',
        'memory_deleted': 'Memory deleted',
        'session_deleted': 'Session deleted',
        'approved': 'Approved',
        'rejected': 'Rejected',
        'batch_approved': 'Batch approved {count} items',
        'risk_config_updated': 'Risk configuration updated',

        # Knowledge services
        'extract_service_unavailable': 'Extraction service unavailable: {e}',
        'vector_service_unavailable': 'Vector service unavailable: {e}',
        'compress_service_unavailable': 'Compression service unavailable: {e}',
        'export_service_unavailable': 'Export service unavailable: {e}',
        'analysis_service_unavailable': 'Analysis service temporarily unavailable. Domain has {n} items.',
        'rag_feedback_service_unavailable': 'RAG feedback service unavailable: {e}',
        'rag_stats_service_unavailable': 'RAG stats service unavailable: {e}',
        'memory_service_unavailable': 'Memory service unavailable: {e}',

        # CLI Export
        'api_key_required': 'API Key is required',
        'cli_desc_claude_code': 'Official Anthropic coding assistant',
        'cli_desc_codex': 'OpenAI coding assistant',
        'cli_desc_openclaw': 'AI coding assistant',
        'cli_desc_hermes': 'Hermes AI assistant',
        'cli_desc_cursor': 'AI coding IDE',
        'cli_desc_continue': 'VS Code AI plugin',
        'cli_instructions_cursor': 'In Cursor settings: set OpenAI API Key to {api_key}, Base URL to {base_url}/v1',

        # Additional missing keys
        'pricing_batch_update_done': 'Batch update completed: {created} created, {updated} updated',
        'org_id_required': 'org_id is required',
        'cannot_delete_seed_setting': 'Cannot delete seed setting {key}',
        'rule_not_found': 'Rule not found',
        'channel_not_found': 'Channel not found',
        'model_not_found_named': 'Model {model} not found',
        'alias_not_found_named': 'Alias {alias} not found',
        'model_alias_exists': 'Alias {alias} already exists, use PUT to update',

        # Miscellaneous
        'hello': 'Hello',
        'help_analyze_code': 'Help me analyze the issues in this code',
        'proxy_reorder_hint': 'When enabled, wr-proxy will reorder the messages array in the request body.',
        'proxy_reorder_detail': 'Messages containing dynamic content like URLs, dates, and numbers are moved to the end of the same role group.',
        'anomaly_request_detail': 'Error request details — filterable by error_type / provider / model',
    },

    'zh-CN': {
        # Auth
        'login_success': '登录成功',
        'logout_success': '已登出',
        'invalid_credentials': '用户名或密码错误',
        'username_password_required': '用户名和密码不能为空',
        'wrong_password': '密码错误',
        'password_changed': '密码已修改',
        'new_password_min_length': '新密码至少需要8个字符',

        # Generic CRUD
        'created': '创建成功',
        'updated': '更新成功',
        'deleted': '删除成功',
        'create_failed': '创建失败',
        'update_failed': '更新失败',
        'delete_failed': '删除失败',
        'not_found': '未找到',
        'already_exists': '已存在',
        'no_data': '暂无数据',

        # Validation
        'name_required': '名称不能为空',
        'url_required': 'URL 不能为空',
        'key_required': 'Key 不能为空',
        'model_required': '模型不能为空',
        'password_required': '密码不能为空',
        'pattern_required': '正则不能为空',
        'channel_name_required': '渠道名称不能为空',
        'rule_name_required': '规则名称不能为空',
        'domain_code_required': 'domain_code 不能为空',
        'domain_name_required': 'domain_code 和 domain_name 不能为空',
        'invalid_format_dept_name_email': '格式不正确，需要: 部门 姓名 email',
        'invalid_expires_at': 'expires_at 格式无效',
        'invalid_org_type': 'org_type 必须为 company/department/group',
        'invalid_org_type_value': 'org_type 无效',
        'invalid_type': 'type 必须为 {VALID_TYPES}',
        'invalid_level': 'level 必须为 {VALID_LEVELS}',
        'invalid_tier': 'tier 必须是 {VALID_TIERS} 之一',
        'invalid_category': 'category 必须为 {VALID_CATEGORIES}',
        'invalid_desensitize_level': 'desensitize_level 必须为 off/standard/strict',
        'invalid_regex': '正则语法错误: {e}',
        'unsupported_type': '不支持的类型: {provider_type}',

        # Field validation
        'field_required_model': 'model 不能为空',
        'field_required_alias_target': 'alias 和 target 不能为空',
        'field_required_target': 'target 不能为空',
        'field_required_target_code': 'target_code 不能为空',
        'field_required_ids': 'ids 不能为空',
        'field_required_token_id': 'token_id 不能为空',
        'field_required_text': 'text 字段不能为空',
        'field_required_channels_array': 'channels 数组不能为空',
        'field_required_items_array': 'items 数组不能为空',

        # Provider
        'provider_created': '供应商已创建',
        'provider_updated': '供应商已更新',
        'provider_deleted': '供应商已删除',
        'provider_not_found': '供应商未找到',
        'base_url_required': 'Base URL 不能为空',

        # Channel
        'channel_created': '渠道已创建',
        'channel_updated': '渠道已更新',
        'channel_batch_created': '批量创建 {len} 个渠道',
        'channel_batch_updated': '批量更新完成: {created} 新增, {updated} 更新',

        # Token / Member
        'token_created': 'Token 已创建',
        'token_updated': 'Token 已更新',
        'token_deleted': 'Token 已删除',
        'token_not_found': 'Token 未找到',
        'member_created': '成员已创建',
        'member_updated': '成员已更新',
        'member_not_found': '成员未找到',
        'member_transferred': '成员已转移',

        # Org
        'org_created': '组织已创建',
        'org_updated': '组织已更新',
        'org_deleted': '组织已删除',
        'org_not_found': '组织未找到',
        'org_name_required': '组织名称不能为空',
        'org_id_not_found': '组织 ID {org_id} 不存在',
        'parent_org_not_found': '父组织未找到',
        'target_org_not_found': '目标组织未找到',
        'cannot_set_self_as_parent': '不能将自身设为父组织',
        'org_has_children': '该组织下有子组织，请先删除子组织',
        'org_has_members': '该组织下仍有成员，请先移除',

        # Batch import
        'batch_import_done': '批量导入完成: 成功 {success} 个, 失败 {failed} 个',
        'members_array_or_text_required': 'members 数组或文本不能为空',

        # Settings
        'settings_updated': '设置已更新',
        'settings_updated_count': '已更新 {len} 项设置',
        'setting_not_found': '设置 {key} 未找到',
        'setting_not_editable': '{key}: 该设置不可编辑',
        'settings_initialized': '已初始化 {len} 项',
        'db_commit_failed': '数据库提交失败: {e}',

        # Backup
        'backup_sqlite_only': '备份仅支持 SQLite',
        'restore_sqlite_only': '恢复仅支持 SQLite',
        'backup_file_not_found': '备份文件未找到',
        'restored': '已恢复',

        # SMTP
        'smtp_no_server': 'SMTP 服务器未配置',
        'smtp_no_recipient': '收件地址未配置',
        'smtp_no_credentials': 'SMTP 用户名或密码未配置',
        'smtp_test_sent': '测试邮件已发送至 {email_to}',
        'smtp_auth_failed': 'SMTP 认证失败: {e}\n提示：QQ 邮箱请使用授权码，而非登录密码。',
        'smtp_connection_failed': 'SMTP 连接失败: {e}',
        'smtp_sender_rejected': '发件人地址被拒绝: {e}',
        'smtp_recipient_rejected': '收件人地址被拒绝: {e}',
        'smtp_connection_closed': 'SMTP 连接异常关闭，可能是认证失败或被服务器拒绝。\n提示：QQ 邮箱请使用授权码（在 设置 → 账户 → POP3/IMAP/SMTP 中获取），而非登录密码。',
        'smtp_send_failed': '邮件发送失败: {e}',
        'network_error': '网络连接失败: {err_msg}',
        'unknown_error': '未知错误: {e}',

        # Proxy
        'proxy_reloaded': 'wr-proxy 已重载',
        'proxy_reload_error': 'wr-proxy 返回 {status}: {error}',
        'proxy_unreachable': '无法连接 wr-proxy（{proxy_url}），请确认 wr-proxy 正在运行',
        'proxy_reload_failed': '重载失败: {e}',
        'request_failed': '请求失败',
        'request_failed_detail': '请求失败: {e}',
        'demo_test_response': '这是 WebRouter Demo 的模拟响应。API 测试功能运行正常 — 在正式环境中，请求将被转发到真实的上游 AI 服务。',
        'no_available_provider': '没有可用的 Provider 处理模型 "{model}"。可能原因：1) Provider 状态为 auth_failed/不健康（系统会自动退避重试）；2) 模型未在任何启用的 Provider 中配置；3) 所有 Provider 都在冷却中。可在"系统设置"中重载 wr-proxy 强制刷新。',

        # Refresh / Cache
        'refresh_sent': '刷新请求已发送',
        'cooldown_clear_sent': '冷却清除请求已发送',

        # Pricing
        'pricing_created': '定价已创建',
        'pricing_updated': '定价已更新',
        'pricing_already_exists': '{model} 已存在，请用 PUT 更新',
        'cannot_delete_default_pricing': '无法删除默认定价',

        # Model grades
        'model_grade_created': '模型等级已创建',
        'model_grade_updated': '模型等级已更新',

        # Model aliases
        'model_alias_created': '模型别名已创建',
        'model_alias_updated': '模型别名已更新',

        # Desensitization
        'desensitize_rule_created': '脱敏规则已创建',
        'desensitize_rule_updated': '脱敏规则已更新',
        'desensitize_rule_deleted': '脱敏规则已删除',

        # Quota
        'quota_updated': '配额已更新',
        'quota_reset': '配额已重置',

        # Knowledge
        'knowledge_activated': '知识库已开通',
        'knowledge_confirm_activation': '请确认开通知识库',
        'domain_created': '业务域已创建',
        'domain_updated': '业务域已更新',
        'domain_confirmed': '业务域已确认',
        'domain_code_exists': '域代码 {code} 已存在',
        'target_domain_not_found': '目标域 {target_code} 不存在',
        'merged_to_domain': '已合并到 {domain}，迁移 {migrated} 条知识',
        'memory_updated': '记忆已更新',
        'memory_deleted': '记忆已删除',
        'session_deleted': '会话已删除',
        'approved': '已通过',
        'rejected': '已拒绝',
        'batch_approved': '已批量通过 {count} 条',
        'risk_config_updated': '风险配置已更新',

        # Knowledge services
        'extract_service_unavailable': '提取服务不可用: {e}',
        'vector_service_unavailable': '向量服务不可用: {e}',
        'compress_service_unavailable': '压缩服务不可用: {e}',
        'export_service_unavailable': '导出服务不可用: {e}',
        'analysis_service_unavailable': '分析服务暂不可用，该域有 {n} 条知识。',
        'rag_feedback_service_unavailable': 'RAG 反馈服务不可用: {e}',
        'rag_stats_service_unavailable': 'RAG 统计服务不可用: {e}',
        'memory_service_unavailable': '记忆服务不可用: {e}',

        # CLI Export
        'api_key_required': 'API Key 不能为空',
        'cli_desc_claude_code': 'Anthropic 官方编程助手',
        'cli_desc_codex': 'OpenAI 编程助手',
        'cli_desc_openclaw': 'AI 编程助手',
        'cli_desc_hermes': 'Hermes AI 助手',
        'cli_desc_cursor': 'AI 编程 IDE',
        'cli_desc_continue': 'VS Code AI 插件',
        'cli_instructions_cursor': '在 Cursor 设置中：OpenAI API Key 填 {api_key}，Base URL 填 {base_url}/v1',

        # Additional missing keys
        'pricing_batch_update_done': '批量更新完成: {created} 新增, {updated} 更新',
        'org_id_required': '需要 org_id',
        'cannot_delete_seed_setting': '无法删除种子设置 {key}',
        'rule_not_found': '规则未找到',
        'channel_not_found': '渠道未找到',
        'model_not_found_named': '模型 {model} 未找到',
        'alias_not_found_named': '别名 {alias} 未找到',
        'model_alias_exists': '别名 {alias} 已存在，请用 PUT 更新',

        # Miscellaneous
        'hello': '你好',
        'help_analyze_code': '帮我分析这段代码的问题',
        'proxy_reorder_hint': '开启后 wr-proxy 会对请求 body 中的 messages 数组重新排序，',
        'proxy_reorder_detail': '把包含 URL、日期、数字等动态内容的 message 移到同 role 组的最后。',
        'anomaly_request_detail': '异常请求明细 — 支持按 error_type / provider / model 筛选',
    }
}


def get_message(key, lang_or_request='zh-CN'):
    """Get translated message. Accepts a lang string or a Flask request object."""
    lang = lang_or_request
    if hasattr(lang_or_request, 'headers'):
        al = lang_or_request.headers.get('Accept-Language', '')
        lang = 'en' if al.startswith('en') else 'zh-CN'
    return MESSAGES.get(lang, {}).get(key, key)


def get_lang(request):
    """Parse Accept-Language header. Returns 'en' or 'zh-CN'."""
    return 'en' if request.headers.get('Accept-Language', '').startswith('en') else 'zh-CN'
