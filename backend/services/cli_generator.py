"""CLI配置生成器 — 为各类AI CLI工具生成配置文件/环境变量"""
import json, logging

logger = logging.getLogger(__name__)


class CLIGenerator:
    """根据工具标识生成对应的CLI配置，返回统一格式dict"""

    SUPPORTED_TOOLS = ("claude-code", "codex", "hermes", "openclaw")

    def generate_config(self, tool_id: str, base_url: str, api_key: str) -> dict:
        """统一入口：根据tool_id分发到对应生成方法

        Args:
            tool_id: 工具标识 (claude-code/codex/hermes/openclaw)
            base_url: API基础地址，如 https://api.example.com
            api_key: 用户API Key

        Returns:
            dict: 包含 shell/yaml/json/instructions 等键的配置字典
        """
        handler = {
            "claude-code": self._claude_code,
            "codex": self._codex,
            "hermes": self._hermes,
            "openclaw": self._openclaw,
        }.get(tool_id)
        if not handler:
            return {"error": f"不支持的CLI工具: {tool_id}，可选: {self.SUPPORTED_TOOLS}"}
        return handler(base_url, api_key)

    # ── Claude Code ───────────────────────────────────────

    @staticmethod
    def _claude_code(base_url: str, api_key: str) -> dict:
        """生成shell环境变量: ANTHROPIC_API_KEY, ANTHROPIC_BASE_URL"""
        shell = (
            f'export ANTHROPIC_API_KEY="{api_key}"\n'
            f'export ANTHROPIC_BASE_URL="{base_url}"\n'
        )
        return {"shell": shell, "instructions":
                "添加到 ~/.bashrc 或 ~/.zshrc，source后运行 claude 即可。"}

    # ── OpenAI Codex ──────────────────────────────────────

    @staticmethod
    def _codex(base_url: str, api_key: str) -> dict:
        """生成JSON配置: ~/.codex/config.json"""
        config = {"apiKey": api_key, "apiBaseUrl": base_url}
        return {"json": json.dumps(config, indent=2), "instructions":
                "将JSON写入 ~/.codex/config.json (mkdir -p ~/.codex)"}

    # ── Hermes ────────────────────────────────────────────

    @staticmethod
    def _hermes(base_url: str, api_key: str) -> dict:
        """生成YAML配置: ~/.hermes/config.yaml"""
        yaml_str = (
            f"model:\n"
            f"  base_url: \"{base_url}\"\n"
            f"  api_key: \"{api_key}\"\n"
        )
        return {"yaml": yaml_str, "instructions":
                "将内容写入 ~/.hermes/config.yaml (mkdir -p ~/.hermes)"}

    # ── OpenClaw ──────────────────────────────────────────

    @staticmethod
    def _openclaw(base_url: str, api_key: str) -> dict:
        """生成shell环境变量: OPENCLAW_API_KEY, OPENCLAW_BASE_URL"""
        shell = (
            f'export OPENCLAW_API_KEY="{api_key}"\n'
            f'export OPENCLAW_BASE_URL="{base_url}"\n'
        )
        return {"shell": shell, "instructions":
                "添加到 ~/.bashrc 或 ~/.zshrc，source后OpenClaw自动读取环境变量。"}
