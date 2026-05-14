#!/usr/bin/env python3
"""
脱敏标记格式实测脚本
测试大模型对不同脱敏标记格式的识别和保留能力

测试维度:
1. 标记是否被保留（不丢失、不篡改）
2. 标记是否影响模型理解上下文
3. 标记是否被模型"修复"或改写

测试的标记格式:
A. Unicode书名号: 〈NAME_1〉〈PHONE_1〉
B. 方括号编号: [NAME_1] [PHONE_1]  
C. 尖括号: <NAME_1> <PHONE_1>
D. 管道符: |NAME_1| |PHONE_1|
E. 自然语言占位: 某人 某手机号
F. 掩码: 张* 138****1234
"""

import json
import requests
import time
import sys

# wr-proxy 配置
BASE_URL = "http://127.0.0.1:5051/v1/chat/completions"
API_KEY = ""  # 从命令行或环境变量读取

TEST_PROMPTS = [
    {
        "name": "格式A: Unicode书名号",
        "prompt": "我叫〈NAME_1〉，电话是〈PHONE_1〉，在〈COMPANY_1〉工作。请用一句话复述我的个人信息。",
        "desc": "测试〈〉书名号标记"
    },
    {
        "name": "格式B: 方括号编号",
        "prompt": "我叫[NAME_1]，电话是[PHONE_1]，在[COMPANY_1]工作。请用一句话复述我的个人信息。",
        "desc": "测试[]方括号标记"
    },
    {
        "name": "格式C: 尖括号",
        "prompt": "我叫<NAME_1>，电话是<PHONE_1>，在<COMPANY_1>工作。请用一句话复述我的个人信息。",
        "desc": "测试<>尖括号标记"
    },
    {
        "name": "格式D: 管道符",
        "prompt": "我叫|NAME_1|，电话是|PHONE_1|，在|COMPANY_1|工作。请用一句话复述我的个人信息。",
        "desc": "测试||管道符标记"
    },
    {
        "name": "格式E: 自然语言占位",
        "prompt": "我叫某人，电话是某手机号，在某公司工作。请用一句话复述我的个人信息。",
        "desc": "测试自然语言占位"
    },
    {
        "name": "格式F: 掩码",
        "prompt": "我叫张*，电话是138****1234，在华为*工作。请用一句话复述我的个人信息。",
        "desc": "测试部分掩码"
    },
    {
        "name": "混合格式: 最接近实际使用",
        "prompt": "客户〈CUST_1〉反馈，订单号〈ORDER_1〉的手机〈PHONE_1〉收不到验证码，请帮忙排查。注意不要拨打客户电话，只需要说明排查步骤。",
        "desc": "测试实际场景+书名号"
    },
    {
        "name": "混合格式B: 实际场景方括号",
        "prompt": "客户[CUST_1]反馈，订单号[ORDER_1]的手机[PHONE_1]收不到验证码，请帮忙排查。注意不要拨打客户电话，只需要说明排查步骤。",
        "desc": "测试实际场景+方括号"
    },
    {
        "name": "代码场景: 敏感配置",
        "prompt": "数据库连接串是〈DB_URL_1〉，API密钥是〈API_KEY_1〉。请帮我写一个Python函数来安全地读取这些配置。",
        "desc": "测试代码场景+书名号"
    },
    {
        "name": "代码场景: 敏感配置方括号",
        "prompt": "数据库连接串是[DB_URL_1]，API密钥是[API_KEY_1]。请帮我写一个Python函数来安全地读取这些配置。",
        "desc": "测试代码场景+方括号"
    },
]

def call_model(prompt, model="qwen3-coder-flash"):
    """调用 wr-proxy"""
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {API_KEY}"
    }
    payload = {
        "model": model,
        "messages": [{"role": "user", "content": prompt}],
        "max_tokens": 512,
        "temperature": 0.3,
    }
    try:
        resp = requests.post(BASE_URL, headers=headers, json=payload, timeout=30)
        data = resp.json()
        if "choices" in data:
            return data["choices"][0]["message"]["content"]
        else:
            return f"[ERROR] {json.dumps(data, ensure_ascii=False)}"
    except Exception as e:
        return f"[EXCEPTION] {e}"


def evaluate_response(prompt, response, markers):
    """评估模型响应中标记的保留情况"""
    results = {}
    for marker in markers:
        if marker in response:
            results[marker] = "保留"
        else:
            # 检查是否被改写（去掉特殊字符后匹配）
            core = marker.strip("〈〉[]<>|")
            if core in response:
                results[marker] = f"改写(含{core})"
            else:
                results[marker] = "丢失"
    return results


def main():
    global API_KEY
    
    if len(sys.argv) > 1:
        API_KEY = sys.argv[1]
    else:
        API_KEY = input("请输入API Key (sk-wr-...): ").strip()
    
    if not API_KEY:
        print("错误: 需要API Key")
        sys.exit(1)
    
    print("=" * 70)
    print("脱敏标记格式实测 — 测试大模型对不同标记的识别和保留能力")
    print("=" * 70)
    
    results = []
    
    for i, test in enumerate(TEST_PROMPTS):
        print(f"\n{'─' * 70}")
        print(f"测试 {i+1}/{len(TEST_PROMPTS)}: {test['name']}")
        print(f"说明: {test['desc']}")
        print(f"输入: {test['prompt']}")
        print(f"调用中...", end=" ", flush=True)
        
        response = call_model(test["prompt"])
        print("完成")
        
        # 提取标记
        import re
        markers_angular = re.findall(r'〈\w+〉', test["prompt"])
        markers_square = re.findall(r'\[\w+\]', test["prompt"])
        markers_angle = re.findall(r'<\w+>', test["prompt"])
        markers_pipe = re.findall(r'\|\w+\|', test["prompt"])
        markers = markers_angular + markers_square + markers_angle + markers_pipe
        
        eval_result = evaluate_response(test["prompt"], response, markers) if markers else {}
        
        print(f"输出: {response[:200]}{'...' if len(response)>200 else ''}")
        if eval_result:
            print(f"标记保留: {eval_result}")
        
        results.append({
            "name": test["name"],
            "prompt": test["prompt"],
            "response": response,
            "markers": eval_result,
        })
        
        time.sleep(1)  # 避免限流
    
    # 汇总
    print("\n" + "=" * 70)
    print("汇总结果")
    print("=" * 70)
    
    for r in results:
        marker_status = ", ".join(f"{k}→{v}" for k, v in r["markers"].items()) if r["markers"] else "无标记"
        print(f"  {r['name']}: {marker_status}")


if __name__ == "__main__":
    main()
