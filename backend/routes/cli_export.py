# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""CLI config export API — export WR Token for various CLI tools."""
import json
from flask import Blueprint, jsonify, request
from i18n.messages import get_message

cli_bp = Blueprint('cli_export', __name__)

# Supported tools and their config templates. `description` and `instructions`
# hold i18n keys resolved per-request via get_message().
CLI_TOOLS = {
    'claude-code': {
        'name': 'Claude Code',
        'description': 'cli_desc_claude_code',
        'env_vars': {
            'ANTHROPIC_API_KEY': '{api_key}',
            'ANTHROPIC_BASE_URL': '{base_url}/v1',
        },
        'shell_export': 'export ANTHROPIC_API_KEY="***"\nexport ANTHROPIC_BASE_URL="{base_url}/v1"',
    },
    'codex': {
        'name': 'OpenAI Codex',
        'description': 'cli_desc_codex',
        'env_vars': {
            'OPENAI_API_KEY': '{api_key}',
            'OPENAI_BASE_URL': '{base_url}/v1',
        },
        'shell_export': 'export OPENAI_API_KEY="***"\nexport OPENAI_BASE_URL="{base_url}/v1"',
    },
    'openclaw': {
        'name': 'OpenClaw',
        'description': 'cli_desc_openclaw',
        'env_vars': {
            'OPENAI_API_KEY': '{api_key}',
            'OPENAI_BASE_URL': '{base_url}/v1',
        },
        'shell_export': 'export OPENAI_API_KEY="***"\nexport OPENAI_BASE_URL="{base_url}/v1"',
    },
    'hermes': {
        'name': 'Hermes Agent',
        'description': 'cli_desc_hermes',
        'config_yaml': (
            'providers:\n'
            '  openai:\n'
            '    api_key: "{api_key}"\n'
            '    base_url: "{base_url}/v1"\n'
        ),
    },
    'cursor': {
        'name': 'Cursor IDE',
        'description': 'cli_desc_cursor',
        'instructions_key': 'cli_instructions_cursor',
    },
    'continue': {
        'name': 'Continue',
        'description': 'cli_desc_continue',
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
    """List supported CLI tools."""
    tools = []
    for key, conf in CLI_TOOLS.items():
        tools.append({
            'id': key,
            'name': conf['name'],
            'description': get_message(conf['description'], request),
        })
    return jsonify({'tools': tools})


@cli_bp.route('/export/<tool_id>')
def export_config(tool_id):
    """Export config for the specified tool."""
    if tool_id not in CLI_TOOLS:
        return jsonify({'error': f'Unsupported tool: {tool_id}'}), 404

    tool = CLI_TOOLS[tool_id]
    base_url = request.host_url.rstrip('/')
    api_key = request.args.get('api_key', 'sk-you...here')

    result = {
        'tool': tool_id,
        'name': tool['name'],
    }

    if 'shell_export' in tool:
        result['shell'] = tool['shell_export'].format(
            api_key=api_key, base_url=base_url
        )

    if 'env_vars' in tool:
        result['env_vars'] = {
            k: v.format(api_key=api_key, base_url=base_url)
            for k, v in tool['env_vars'].items()
        }

    if 'config_yaml' in tool:
        result['yaml'] = tool['config_yaml'].format(
            api_key=api_key, base_url=base_url
        )

    if 'config_json' in tool:
        config_str = json.dumps(tool['config_json'])
        result['json'] = config_str.replace('{api_key}', api_key).replace('{base_url}', base_url)

    if 'instructions_key' in tool:
        result['instructions'] = get_message(tool['instructions_key'], request).format(
            api_key=api_key, base_url=base_url
        )

    return jsonify(result)


@cli_bp.route('/test', methods=['POST'])
def test_connection():
    """Test API connectivity."""
    data = request.get_json() or {}
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
