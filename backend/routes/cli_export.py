"""CLI配置导出API"""
import json
from flask import Blueprint, jsonify
from models.newapi_adapter import NewAPIAdapter

cli_bp = Blueprint('cli_export', __name__)

# 支持的工具及其配置模板
CLI_TOOLS = {
    'claude-code': {
        'name': 'Claude Code',
        'description': 'Anthropic官方编程助手',
        'env_vars': {
            'ANTHROPIC_API_KEY': '{api_key}',
            'ANTHROPIC_BASE_URL': '{base_url}/v1',
        },
        'shell_export': 'export ANTHROPIC_API_KEY="{api_key}"\nexport ANTHROPIC_BASE_URL="{base_url}/v1"',
    },
    'codex': {
        'name': 'OpenAI Codex',
        'description': 'OpenAI编程助手',
        'env_vars': {
            'OPENAI_API_KEY': '{api_key}',
            'OPENAI_BASE_URL': '{base_url}/v1',
        },
        'shell_export': 'export OPENAI_API_KEY="{api_key}"\nexport OPENAI_BASE_URL="{base_url}/v1"',
    },
    'openclaw': {
        'name': 'OpenClaw',
        'description': 'AI编程助手',
        'env_vars': {
            'OPENAI_API_KEY': '{api_key}',
            'OPENAI_BASE_URL': '{base_url}/v1',
        },
        'shell_export': 'export OPENAI_API_KEY="{api_key}"\nexport OPENAI_BASE_URL="{base_url}/v1"',
    },
    'hermes': {
        'name': 'Hermes Agent',
        'description': 'Hermes AI助手',
        'config_yaml': (
            'providers:\n'
            '  openai:\n'
            '    api_key: "{api_key}"\n'
            '    base_url: "{base_url}/v1"\n'
        ),
    },
    'cursor': {
        'name': 'Cursor IDE',
        'description': 'AI编程IDE',
        'instructions': '在Cursor设置中：OpenAI API Key填 {api_key}，Base URL填 {base_url}/v1',
    },
    'continue': {
        'name': 'Continue',
        'description': 'VS Code AI插件',
        'config_json': {
            'models': [{
                'title': 'WebRouter',
                'provider': 'openai',
                'model': 'auto',
                'apiKey': '{api_key}',
                'apiBase': '{base_url}/v1',
            }]
        },
    },
}


@cli_bp.route('/tools')
def list_tools():
    """列出支持的CLI工具"""
    tools = []
    for key, conf in CLI_TOOLS.items():
        tools.append({
            'id': key,
            'name': conf['name'],
            'description': conf['description'],
        })
    return jsonify({'tools': tools})


@cli_bp.route('/export/<tool_id>')
def export_config(tool_id):
    """导出指定工具的配置"""
    from flask import request

    if tool_id not in CLI_TOOLS:
        return jsonify({'error': f'Unsupported tool: {tool_id}'}), 404

    tool = CLI_TOOLS[tool_id]
    base_url = request.host_url.rstrip('/')
    api_key = request.args.get('api_key', 'sk-your-api-key-here')

    result = {
        'tool': tool_id,
        'name': tool['name'],
    }

    # Shell export
    if 'shell_export' in tool:
        result['shell'] = tool['shell_export'].format(
            api_key=api_key, base_url=base_url
        )

    # 环境变量
    if 'env_vars' in tool:
        result['env_vars'] = {
            k: v.format(api_key=api_key, base_url=base_url)
            for k, v in tool['env_vars'].items()
        }

    # YAML配置
    if 'config_yaml' in tool:
        result['yaml'] = tool['config_yaml'].format(
            api_key=api_key, base_url=base_url
        )

    # JSON配置
    if 'config_json' in tool:
        config_str = json.dumps(tool['config_json'])
        result['json'] = config_str.replace('{api_key}', api_key).replace('{base_url}', base_url)

    # 使用说明
    if 'instructions' in tool:
        result['instructions'] = tool['instructions'].format(
            api_key=api_key, base_url=base_url
        )

    return jsonify(result)


@cli_bp.route('/test', methods=['POST'])
def test_connection():
    """测试API连接"""
    from flask import request as req
    data = req.get_json() or {}
    base_url = data.get('base_url', '')
    api_key = data.get('api_key', '')

    if not base_url or not api_key:
        return jsonify({'error': 'base_url and api_key required'}), 400

    import requests as http
    try:
        resp = http.get(
            f"{base_url}/v1/models",
            headers={'Authorization': f'Bearer {api_key}'},
            timeout=10,
        )
        if resp.status_code == 200:
            models = resp.json().get('data', [])
            return jsonify({
                'status': 'ok',
                'model_count': len(models),
                'models': [m.get('id', '') for m in models[:20]],
            })
        else:
            return jsonify({
                'status': 'error',
                'code': resp.status_code,
                'message': resp.text[:200],
            })
    except Exception as e:
        return jsonify({'status': 'error', 'message': str(e)})
