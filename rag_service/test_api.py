import json
import requests

url = "http://127.0.0.1:18081/ask"
payload = {
    "query": "连续签到中断后怎么办",
    "top_k": 3,
    "use_llm": True
}

proxies = {
    "http": None,
    "https": None,
}

print("正在请求 /ask ...")
try:
    response = requests.post(url, json=payload, proxies=proxies, timeout=10)
    print(f"HTTP 状态码: {response.status_code}")
    print(json.dumps(response.json(), indent=2, ensure_ascii=False))
except requests.exceptions.Timeout:
    print("❌ 请求超时，请检查服务是否启动或外部 LLM 是否不可达。")
except requests.exceptions.ConnectionError:
    print("❌ 连接失败，请确认 rag_service 和 qdrant 已启动，端口是否正确。")
except Exception as e:
    print(f"❌ 未知错误: {e}")
