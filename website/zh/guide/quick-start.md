---
title: 快速开始
description: 几分钟内让 WebRouter 运行起来
---

# 快速开始

## 在线体验

不想安装？立即访问 **[在线演示](https://webrouter-demo.fly.dev)** 体验 WebRouter 管理面板（账号 `demo` / 密码 `demo123456`）。

## 一键安装

```bash
git clone https://github.com/<org>/webrouter.git
cd webrouter
bash deploy/install.sh
```

安装脚本自动完成：
1. 检测操作系统和 CPU 架构
2. 安装 Python 3.8+（如缺失）
3. 创建虚拟环境并安装依赖
4. 编译 wr-proxy Go 网关（如 Go 可用）
5. 生成配置文件和启动脚本
6. 启动服务

## 首次登录

打开 `http://localhost:5050`，使用以下账号登录：

- **用户名：** `admin`
- **密码：** `admin123456`

## Docker 部署

```bash
cd webrouter
docker compose -f deploy/docker-compose.yml up -d
```

## 添加第一个数据源

1. 进入 **数据源** → **+ 添加**
2. 选择类型 `direct`，填入 OpenAI 的 Base URL 和 API Key
3. 点击 **检测** 验证连通性
4. 网关地址：`http://localhost:5051/v1/chat/completions`