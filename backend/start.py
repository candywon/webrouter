#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 Jianlin Huang <https://webrouter.tech>
# SPDX-License-Identifier: BUSL-1.1

"""
WebRouter 进程管理器 — 启动/停止/重启/状态查询
管理 WebRouter (Flask) + wr-proxy (Go sidecar) 双进程

用法:
  python3 start.py start     启动所有服务
  python3 start.py stop      停止所有服务
  python3 start.py restart   重启所有服务
  python3 start.py status    查看运行状态
  python3 start.py logs      查看实时日志
  python3 start.py logstats  查看日志文件大小
  python3 start.py cleanlogs 轮转/清理日志文件
"""

import os
import sys
import signal
import subprocess
import time
import gzip
import shutil
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
PROXY_DIR = BASE_DIR / "wr-proxy"

# 默认配置
WEBROUTER_PORT = int(os.environ.get("WEBROUTER_PORT", "5050"))
PROXY_PORT = int(os.environ.get("WR_PROXY_PORT", "5051"))
FLASK_HOST = os.environ.get("FLASK_HOST", "0.0.0.0")

# 日志轮转配置
LOG_MAX_BYTES = 10 * 1024 * 1024  # 10MB
LOG_BACKUP_COUNT = 5  # 最多保留 5 份压缩备份


def rotate_log(log_path):
    """日志文件轮转：超过 MAX_BYTES 时压缩旧日志，保留最新 N 份"""
    if not log_path.exists():
        return

    size = log_path.stat().st_size
    if size < LOG_MAX_BYTES:
        return

    print(f"  日志 {log_path.name} 达到 {size // 1024}KB，执行轮转...")

    # 删除最旧的备份
    oldest = log_path.with_suffix(f".log.{LOG_BACKUP_COUNT}.gz")
    if oldest.exists():
        oldest.unlink()

    # 依次重命名: .log.4.gz -> .log.5.gz, .log.3.gz -> .log.4.gz, ...
    for i in range(LOG_BACKUP_COUNT - 1, 0, -1):
        src = log_path.with_suffix(f".log.{i}.gz")
        dst = log_path.with_suffix(f".log.{i + 1}.gz")
        if src.exists():
            src.rename(dst)

    # 压缩当前日志为 .log.1.gz
    backup = log_path.with_suffix(".log.1.gz")
    with open(log_path, "rb") as f_in, gzip.open(backup, "wb") as f_out:
        shutil.copyfileobj(f_in, f_out)

    # 清空原日志
    log_path.write_text("")
    print(f"  轮转完成: {backup} ({size // 1024}KB → {backup.stat().st_size // 1024}KB)")


def cleanup_old_logs():
    """清理超过保留份数的旧日志备份"""
    for pattern in ["webrouter", "wr-proxy"]:
        base = LOGS_DIR / f"{pattern}.log"
        for i in range(LOG_BACKUP_COUNT + 1, 100):  # 清理超出限额的残余
            old = base.with_suffix(f".log.{i}.gz")
            if old.exists():
                old.unlink()


def ensure_dirs():
    for d in [DATA_DIR, LOGS_DIR, BIN_DIR]:
        d.mkdir(parents=True, exist_ok=True)


def load_env():
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
    pid_file = PID_DIR / f"{name}.pid"
    if pid_file.exists():
        try:
            return int(pid_file.read_text().strip())
        except (ValueError, OSError):
            return None
    return None


def write_pid(name, pid):
    (PID_DIR / f"{name}.pid").write_text(str(pid))


def remove_pid(name):
    (PID_DIR / f"{name}.pid").unlink(missing_ok=True)


def is_alive(pid):
    if pid is None:
        return False
    try:
        os.kill(pid, 0)
        return True
    except (OSError, ProcessLookupError):
        return False


def check_port(port, timeout=3):
    try:
        req = urllib.request.Request(f"http://localhost:{port}/health")
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status == 200
    except Exception:
        return False


def find_proxy_binary():
    """查找 wr-proxy 二进制"""
    # 优先查项目目录
    for name in ["wr-proxy", "wr-proxy.exe"]:
        path = PROXY_DIR / name
        if path.exists():
            return str(path)
    # 查 bin 目录
    for name in ["wr-proxy", "wr-proxy-linux-amd64"]:
        path = BIN_DIR / name
        if path.exists():
            return str(path)
    # 查系统 PATH
    import shutil
    return shutil.which("wr-proxy")


def start_proxy():
    """启动 wr-proxy Go sidecar"""
    pid = read_pid("wr-proxy")
    if pid and is_alive(pid):
        print(f"  wr-proxy 已在运行 (PID {pid})")
        return True

    binary = find_proxy_binary()
    if not binary:
        print("  wr-proxy 二进制未找到，跳过")
        print(f"  编译: cd wr-proxy && go build -o wr-proxy .")
        return True  # 不阻断 WebRouter 启动

    log_file = open(LOGS_DIR / "wr-proxy.log", "a")
    env = os.environ.copy()
    env["WR_DB_PATH"] = str(BACKEND_DIR / "data" / "webrouter.db")
    env.setdefault("WR_PROXY_PORT", str(PROXY_PORT))
    env.setdefault("WR_KNOWLEDGE_CAPTURE", "1")

    proc = subprocess.Popen(
        [binary],
        stdout=log_file,
        stderr=log_file,
        env=env,
        cwd=str(PROXY_DIR),
    )
    write_pid("wr-proxy", proc.pid)
    print(f"  wr-proxy 启动中... (PID {proc.pid}, 端口 {PROXY_PORT})")

    for i in range(15):
        time.sleep(1)
        if check_port(PROXY_PORT):
            print(f"  wr-proxy 就绪 ✓")
            return True
        if not is_alive(proc.pid):
            print(f"  wr-proxy 启动失败！查看日志: {LOGS_DIR / 'wr-proxy.log'}")
            return False

    print(f"  wr-proxy 等待超时（进程存活但端口未响应）")
    return True


def start_webrouter():
    """启动 WebRouter Flask 服务"""
    pid = read_pid("webrouter")
    if pid and is_alive(pid):
        print(f"  WebRouter 已在运行 (PID {pid})")
        return True

    db_uri = os.environ.get("DATABASE_URI", f"sqlite:///{BACKEND_DIR / 'data' / 'webrouter.db'}")
    os.environ["DATABASE_URI"] = db_uri

    log_file = open(LOGS_DIR / "webrouter.log", "a")

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


def stop_proxy():
    """停止 wr-proxy"""
    pid = read_pid("wr-proxy")
    if not pid or not is_alive(pid):
        remove_pid("wr-proxy")
        print("  wr-proxy 未在运行")
        return

    print(f"  停止 wr-proxy (PID {pid})...")
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

    remove_pid("wr-proxy")
    print("  wr-proxy 已停止 ✓")


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
    print("")
    print("=" * 50)
    print("  WebRouter 服务状态")
    print("=" * 50)

    for name, port, label in [
        ("wr-proxy", PROXY_PORT, "wr-proxy (Go)"),
        ("webrouter", WEBROUTER_PORT, "WebRouter (Flask)"),
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
    print(f"  API 代理: http://localhost:{PROXY_PORT}/v1/chat/completions")
    print(f"  日志目录: {LOGS_DIR}")
    print("")


def tail_logs():
    import shutil
    tail = shutil.which("tail")
    if tail:
        log_files = [str(LOGS_DIR / "webrouter.log")]
        if (LOGS_DIR / "wr-proxy.log").exists():
            log_files.append(str(LOGS_DIR / "wr-proxy.log"))
        os.execvp(tail, ["tail", "-f"] + log_files)
    else:
        for name in ["webrouter", "wr-proxy"]:
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
        # 启动前检查日志轮转
        cleanup_old_logs()
        rotate_log(LOGS_DIR / "webrouter.log")
        rotate_log(LOGS_DIR / "wr-proxy.log")
        ok1 = start_proxy()
        ok2 = start_webrouter()
        if ok1 and ok2:
            print(f"\n✓ 全部启动成功！")
            print(f"  管理后台: http://localhost:{WEBROUTER_PORT}")
            print(f"  API 代理: http://localhost:{PROXY_PORT}/v1/chat/completions")
        else:
            print("\n⚠ 部分服务启动失败，请检查日志")
            sys.exit(1)

    elif cmd == "stop":
        print("\n停止 WebRouter 服务...")
        stop_webrouter()
        stop_proxy()
        print("\n✓ 全部已停止")

    elif cmd == "restart":
        print("\n重启 WebRouter 服务...")
        stop_webrouter()
        stop_proxy()
        time.sleep(1)
        cleanup_old_logs()
        rotate_log(LOGS_DIR / "webrouter.log")
        rotate_log(LOGS_DIR / "wr-proxy.log")
        ok1 = start_proxy()
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

    elif cmd == "cleanlogs":
        print("\n清理日志文件...")
        cleanup_old_logs()
        rotate_log(LOGS_DIR / "webrouter.log")
        rotate_log(LOGS_DIR / "wr-proxy.log")
        print("  清理完成")

    elif cmd == "logstats":
        print("\n日志文件大小统计:")
        for name in ["webrouter", "wr-proxy"]:
            log = LOGS_DIR / f"{name}.log"
            if log.exists():
                size = log.stat().st_size
                print(f"  {name}.log: {size // 1024}KB")
                # 列出备份文件
                for i in range(1, LOG_BACKUP_COUNT + 1):
                    bak = log.with_suffix(f".log.{i}.gz")
                    if bak.exists():
                        print(f"  {name}.log.{i}.gz: {bak.stat().st_size // 1024}KB")
        print()

    else:
        print(f"未知命令: {cmd}")
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
