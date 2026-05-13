#!/usr/bin/env python3
"""
WebRouter 进程管理器 — 启动/停止/重启/状态查询
管理 WebRouter (Flask) + New-API (sidecar) 双进程

用法:
  python3 start.py start     启动所有服务
  python3 start.py stop      停止所有服务
  python3 start.py restart   重启所有服务
  python3 start.py status    查看运行状态
  python3 start.py logs      查看实时日志
"""

import os
import sys
import signal
import subprocess
import time
import json
import urllib.request
import urllib.error
from pathlib import Path

# 项目根目录
BASE_DIR = Path(__file__).resolve().parent.parent
BIN_DIR = BASE_DIR / "bin"
DATA_DIR = BASE_DIR / "data"
LOGS_DIR = BASE_DIR / "logs"
PID_DIR = LOGS_DIR
BACKEND_DIR = BASE_DIR / "backend"

# 默认配置
WEBROUTER_PORT = int(os.environ.get("WEBROUTER_PORT", "5000"))
NEWAPI_PORT = int(os.environ.get("NEWAPI_PORT", "3000"))
NEWAPI_URL = os.environ.get("NEWAPI_URL", f"http://localhost:{NEWAPI_PORT}")
FLASK_HOST = os.environ.get("FLASK_HOST", "0.0.0.0")


def ensure_dirs():
    """确保必要目录存在"""
    for d in [DATA_DIR, LOGS_DIR, BIN_DIR]:
        d.mkdir(parents=True, exist_ok=True)


def load_env():
    """加载 .env 文件"""
    env_file = BASE_DIR / ".env"
    if env_file.exists():
        with open(env_file) as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith("#") and "=" in line:
                    key, _, val = line.partition("=")
                    key, val = key.strip(), val.strip()
                    if val and key not in os.environ:
                        os.environ[key] = val


def read_pid(name):
    """读取 PID 文件"""
    pid_file = PID_DIR / f"{name}.pid"
    if pid_file.exists():
        try:
            return int(pid_file.read_text().strip())
        except (ValueError, OSError):
            return None
    return None


def write_pid(name, pid):
    """写入 PID 文件"""
    pid_file = PID_DIR / f"{name}.pid"
    pid_file.write_text(str(pid))


def remove_pid(name):
    """删除 PID 文件"""
    pid_file = PID_DIR / f"{name}.pid"
    pid_file.unlink(missing_ok=True)


def is_alive(pid):
    """检查进程是否存活"""
    if pid is None:
        return False
    try:
        os.kill(pid, 0)
        return True
    except (OSError, ProcessLookupError):
        return False


def check_port(port, timeout=3):
    """检查端口是否有服务响应"""
    try:
        req = urllib.request.Request(f"http://localhost:{port}/")
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status == 200
    except Exception:
        return False


def find_newapi_binary():
    """查找 New-API 二进制文件"""
    # 优先查 bin/new-api
    for name in ["new-api", "new-api.exe"]:
        path = BIN_DIR / name
        if path.exists():
            return str(path)
    # 查系统 PATH
    import shutil
    found = shutil.which("new-api")
    return found


def start_newapi():
    """启动 New-API sidecar"""
    pid = read_pid("newapi")
    if pid and is_alive(pid):
        print(f"  New-API 已在运行 (PID {pid})")
        return True

    binary = find_newapi_binary()
    if not binary:
        print("  New-API 二进制未找到，跳过 (使用 Docker 模式请手动启动)")
        print(f"  如需 Docker: docker run -d -p {NEWAPI_PORT}:3000 calciumion/new-api:latest")
        return True  # 不阻断 WebRouter 启动

    log_file = open(LOGS_DIR / "newapi.log", "a")
    env = os.environ.copy()
    env.setdefault("SQL_DSN", str(DATA_DIR / "newapi.db"))
    env.setdefault("PORT", str(NEWAPI_PORT))

    proc = subprocess.Popen(
        [binary],
        stdout=log_file,
        stderr=log_file,
        env=env,
        cwd=str(BASE_DIR),
    )
    write_pid("newapi", proc.pid)
    print(f"  New-API 启动中... (PID {proc.pid}, 端口 {NEWAPI_PORT})")

    # 等待就绪
    for i in range(15):
        time.sleep(1)
        if check_port(NEWAPI_PORT):
            print(f"  New-API 就绪 ✓")
            return True
        if not is_alive(proc.pid):
            print(f"  New-API 启动失败！查看日志: {LOGS_DIR / 'newapi.log'}")
            return False

    print(f"  New-API 等待超时（进程存活但端口未响应）")
    return True


def start_webrouter():
    """启动 WebRouter Flask 服务"""
    pid = read_pid("webrouter")
    if pid and is_alive(pid):
        print(f"  WebRouter 已在运行 (PID {pid})")
        return True

    db_uri = os.environ.get("DATABASE_URI", f"sqlite:///{DATA_DIR / 'webrouter.db'}")
    os.environ["DATABASE_URI"] = db_uri
    os.environ["NEWAPI_URL"] = NEWAPI_URL

    log_file = open(LOGS_DIR / "webrouter.log", "a")

    # 用同一 Python 解释器启动 Flask
    cmd = [
        sys.executable, "-c",
        "from app import create_app; app = create_app(); "
        f"app.run(host='{FLASK_HOST}', port={WEBROUTER_PORT})"
    ]

    proc = subprocess.Popen(
        cmd,
        stdout=log_file,
        stderr=log_file,
        env=os.environ.copy(),
        cwd=str(BACKEND_DIR),
    )
    write_pid("webrouter", proc.pid)
    print(f"  WebRouter 启动中... (PID {proc.pid}, 端口 {WEBROUTER_PORT})")

    # 等待就绪
    for i in range(15):
        time.sleep(1)
        if check_port(WEBROUTER_PORT):
            print(f"  WebRouter 就绪 ✓")
            return True
        if not is_alive(proc.pid):
            print(f"  WebRouter 启动失败！查看日志: {LOGS_DIR / 'webrouter.log'}")
            return False

    print(f"  WebRouter 等待超时")
    return True


def stop_newapi():
    """停止 New-API"""
    pid = read_pid("newapi")
    if not pid or not is_alive(pid):
        remove_pid("newapi")
        print("  New-API 未在运行")
        return

    print(f"  停止 New-API (PID {pid})...")
    try:
        os.kill(pid, signal.SIGTERM)
    except ProcessLookupError:
        pass

    # 等待退出
    for _ in range(10):
        if not is_alive(pid):
            break
        time.sleep(0.5)
    else:
        try:
            os.kill(pid, signal.SIGKILL)
        except ProcessLookupError:
            pass

    remove_pid("newapi")
    print("  New-API 已停止 ✓")


def stop_webrouter():
    """停止 WebRouter"""
    pid = read_pid("webrouter")
    if not pid or not is_alive(pid):
        remove_pid("webrouter")
        print("  WebRouter 未在运行")
        return

    print(f"  停止 WebRouter (PID {pid})...")
    try:
        os.kill(pid, signal.SIGTERM)
    except ProcessLookupError:
        pass

    for _ in range(10):
        if not is_alive(pid):
            break
        time.sleep(0.5)
    else:
        try:
            os.kill(pid, signal.SIGKILL)
        except ProcessLookupError:
            pass

    remove_pid("webrouter")
    print("  WebRouter 已停止 ✓")


def show_status():
    """显示运行状态"""
    print("")
    print("=" * 50)
    print("  WebRouter 服务状态")
    print("=" * 50)

    for name, port, label in [
        ("newapi", NEWAPI_PORT, "New-API"),
        ("webrouter", WEBROUTER_PORT, "WebRouter"),
    ]:
        pid = read_pid(name)
        alive = is_alive(pid) if pid else False
        port_ok = check_port(port) if alive else False
        status = "运行中" if alive else "已停止"
        detail = f"PID {pid}, 端口 {port}" if alive else ""
        health = "响应正常" if port_ok else ("端口无响应" if alive else "")
        icon = "✓" if alive and port_ok else ("⚠" if alive else "✗")

        print(f"  {icon} {label}: {status} {detail} {health}")

    print("")
    print(f"  管理后台: http://localhost:{WEBROUTER_PORT}")
    print(f"  New-API:  http://localhost:{NEWAPI_PORT}")
    print(f"  日志目录: {LOGS_DIR}")
    print("")


def tail_logs():
    """查看实时日志"""
    import shutil
    tail = shutil.which("tail")
    if tail:
        log_files = [str(LOGS_DIR / "webrouter.log")]
        if (LOGS_DIR / "newapi.log").exists():
            log_files.append(str(LOGS_DIR / "newapi.log"))
        os.execvp(tail, ["tail", "-f"] + log_files)
    else:
        # fallback: 读取最后 50 行
        for name in ["webrouter", "newapi"]:
            log = LOGS_DIR / f"{name}.log"
            if log.exists():
                print(f"--- {name}.log (最后50行) ---")
                lines = log.read_text().splitlines()[-50:]
                print("\n".join(lines))


def main():
    ensure_dirs()
    load_env()

    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(0)

    cmd = sys.argv[1].lower()

    if cmd == "start":
        print("\n启动 WebRouter 服务...")
        ok1 = start_newapi()
        ok2 = start_webrouter()
        if ok1 and ok2:
            print(f"\n✓ 全部启动成功！")
            print(f"  管理后台: http://localhost:{WEBROUTER_PORT}")
            print(f"  New-API:  http://localhost:{NEWAPI_PORT}")
        else:
            print("\n⚠ 部分服务启动失败，请检查日志")
            sys.exit(1)

    elif cmd == "stop":
        print("\n停止 WebRouter 服务...")
        stop_webrouter()
        stop_newapi()
        print("\n✓ 全部已停止")

    elif cmd == "restart":
        print("\n重启 WebRouter 服务...")
        stop_webrouter()
        stop_newapi()
        time.sleep(1)
        ok1 = start_newapi()
        ok2 = start_webrouter()
        if ok1 and ok2:
            print("\n✓ 重启完成！")
        else:
            print("\n⚠ 部分服务重启失败")
            sys.exit(1)

    elif cmd == "status":
        show_status()

    elif cmd == "logs":
        tail_logs()

    else:
        print(f"未知命令: {cmd}")
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
