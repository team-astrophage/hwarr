import httpx

BASE_URL = "http://hwarr.com"

resp = httpx.post(f"{BASE_URL}/api/reset", follow_redirects=True)
print(resp.status_code)
try:
    print(resp.json())
except Exception:
    print(resp.text)
