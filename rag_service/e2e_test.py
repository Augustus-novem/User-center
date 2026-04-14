import requests
import os
import time

BASE_URL = "http://127.0.0.1:18081"

# 测试文件配置
TEST_MD_FILE = "demo_upload.txt"
TEST_MD_CONTENT = """签到补签规则
连续签到中断后，会重新开始计算连续天数。
如果产品支持补签，以页面说明为准。"""
TEST_EMPTY_FILE = "empty.txt"
TEST_PDF_FILE = "demo.pdf"


def setup_files():
    print(">>> 初始化测试文件...")
    with open(TEST_MD_FILE, "w", encoding="utf-8") as f:
        f.write(TEST_MD_CONTENT)
    with open(TEST_EMPTY_FILE, "w", encoding="utf-8") as f:
        pass  # 空文件
    with open(TEST_PDF_FILE, "w", encoding="utf-8") as f:
        f.write("fake pdf content")


def cleanup_files():
    print("\n>>> 清理本地测试文件...")
    for file in [TEST_MD_FILE, TEST_EMPTY_FILE, TEST_PDF_FILE]:
        if os.path.exists(file):
            os.remove(file)


def run_tests():
    doc_id = ""
    initial_doc_count = 0

    print("\n=== 开始执行 API 测试 ===")

    # 1. 验证服务启动 (Health Check)
    print("\n1. 测试 /health")
    resp = requests.get(f"{BASE_URL}/health")
    assert resp.status_code == 200, f"Health check 失败: {resp.text}"
    print("✅ 服务存活")

    # 2. 验证文档列表 (获取初始数量)
    print("\n2. 测试 GET /documents (上传前)")
    resp = requests.get(f"{BASE_URL}/documents")
    assert resp.status_code == 200
    initial_doc_count = resp.json().get("total", 0)
    print(f"✅ 当前知识库文档数: {initial_doc_count}")

    # 3. 验证上传并自动入库
    print("\n3. 测试 POST /knowledge/upload")
    with open(TEST_MD_FILE, "rb") as f:
        files = {"file": (TEST_MD_FILE, f, "text/plain")}
        resp = requests.post(f"{BASE_URL}/knowledge/upload", files=files)

    assert resp.status_code == 200, f"上传失败: {resp.text}"
    data = resp.json()
    doc_id = data["document"]["doc_id"]
    print(f"✅ 上传成功, 获分配 doc_id: {doc_id}")

    # 4. 验证上传后文档列表已更新
    print("\n4. 测试 GET /documents (上传后)")
    resp = requests.get(f"{BASE_URL}/documents")
    assert resp.status_code == 200
    new_doc_count = resp.json().get("total", 0)
    assert new_doc_count == initial_doc_count + 1, "文档总数未 +1"

    # 检查列表中是否包含新文档
    docs = resp.json().get("documents", [])
    assert any(d["doc_id"] == doc_id for d in docs), "新文档未出现在列表中"
    print("✅ 文档列表已更新")

    # 给 Qdrant 一点点时间确保索引可读 (通常是实时的，求稳加个延迟)
    time.sleep(1)

    # 5. 验证上传后已经可检索
    print("\n5. 测试 POST /knowledge/search (验证向量入库)")
    payload = {"query": "连续签到中断后怎么计算", "top_k": 3}
    resp = requests.post(f"{BASE_URL}/knowledge/search", json=payload)
    assert resp.status_code == 200
    hits = resp.json().get("hits", [])

    # 验证是否命中了刚才上传的文档
    hit_docs = [hit["doc_id"] for hit in hits]
    assert doc_id in hit_docs, f"未检索到新文档, 返回的 hits: {hit_docs}"
    print("✅ 检索成功，新文档切块已成功向量化并入库")

    # 6. 验证删除接口
    print("\n6. 测试 DELETE /documents/{doc_id}")
    resp = requests.delete(f"{BASE_URL}/documents/{doc_id}")
    assert resp.status_code == 200, f"删除失败: {resp.text}"
    print(f"✅ 删除成功，Qdrant 中被移除的点数量: {resp.json().get('removed_points')}")

    # 7. 验证删除后元数据已移除
    print("\n7. 测试 GET /documents (删除后)")
    resp = requests.get(f"{BASE_URL}/documents")
    assert resp.status_code == 200
    final_doc_count = resp.json().get("total", 0)
    assert final_doc_count == initial_doc_count, "文档总数未恢复到初始状态"
    docs = resp.json().get("documents", [])
    assert not any(d["doc_id"] == doc_id for d in docs), "被删文档仍存在于列表中"
    print("✅ 内存和列表中的文档元数据已清除")

    # 8. 验证删除后索引已移除
    print("\n8. 测试 POST /knowledge/search (验证索引同步删除)")
    resp = requests.post(f"{BASE_URL}/knowledge/search", json=payload)
    assert resp.status_code == 200
    hits = resp.json().get("hits", [])
    hit_docs = [hit["doc_id"] for hit in hits]
    assert doc_id not in hit_docs, "已删除的文档仍被检索到！向量没删干净！"
    print("✅ Qdrant 索引已同步彻底清理")

    # 9. 验证异常路径
    print("\n9. 测试异常处理路径")

    # 9.1 空文件上传
    with open(TEST_EMPTY_FILE, "rb") as f:
        files = {"file": (TEST_EMPTY_FILE, f, "text/plain")}
        resp = requests.post(f"{BASE_URL}/knowledge/upload", files=files)
    assert resp.status_code == 400
    print("✅ 拦截空文件成功 (HTTP 400)")

    # 9.2 不支持的文件格式上传
    with open(TEST_PDF_FILE, "rb") as f:
        files = {"file": (TEST_PDF_FILE, f, "application/pdf")}
        resp = requests.post(f"{BASE_URL}/knowledge/upload", files=files)
    assert resp.status_code == 400
    print("✅ 拦截不支持后缀(PDF)成功 (HTTP 400)")

    # 9.3 删除不存在的文档
    resp = requests.delete(f"{BASE_URL}/documents/doc-not-exists-12345")
    assert resp.status_code == 404
    print("✅ 拦截删除不存在文档成功 (HTTP 404)")

    print("\n🎉 所有 E2E 测试用例全部通过！闭环逻辑稳如老狗。")


if __name__ == "__main__":
    try:
        setup_files()
        run_tests()
    except AssertionError as e:
        print(f"\n❌ 测试失败: {e}")
    except requests.exceptions.ConnectionError:
        print(f"\n❌ 连接失败: 请检查 FastAPI 服务是否在 {BASE_URL} 启动")
    finally:
        cleanup_files()