import requests
import random
import string

BASE_URL = "http://localhost:8080"
ITERATIONS = 20

def log_ok(msg):
    print(f"[OK] {msg}")

def log_fail(msg):
    print(f"[FAIL] {msg}")

def check_status(route, response):
    if response.status_code >= 400:
        log_fail(f"{route} returned {response.status_code}")
    else:
        log_ok(f"{route} returned {response.status_code}")

def random_filename(length=20):
    return ''.join(random.choices(string.ascii_letters + string.digits, k=length))

def test_routes():
    for i in range(ITERATIONS):
        print(f"\n---- Iteration {i+1} ----")

        # GET /
        r = requests.get(f"{BASE_URL}/")
        check_status("GET /", r)

        # GET /admin
        r = requests.get(f"{BASE_URL}/admin")
        check_status("GET /admin", r)

        # GET /settings
        r = requests.get(f"{BASE_URL}/settings")
        check_status("GET /settings", r)

        # POST /upload (valid file)
        with open("./test_exe.exe", "rb") as f:
            r = requests.post(f"{BASE_URL}/upload", files={"file": f})
            check_status("POST /upload (valid)", r)

        # POST /upload (random data)
        random_data = bytes(random.getrandbits(8) for _ in range(100000))
        r = requests.post(
            f"{BASE_URL}/upload",
            files={"file": ("random.bin", random_data)}
        )
        check_status("POST /upload (random)", r)

        # GET /files
        r = requests.get(f"{BASE_URL}/files")
        check_status("GET /files", r)

        # Normal file fetch
        r = requests.get(f"{BASE_URL}/files/test.txt/meta")
        check_status("GET /files/test.txt/meta", r)

        # Path traversal attempt
        r = requests.get(f"{BASE_URL}/files/../../etc/passwd/meta")
        if "root:" in r.text:
            log_fail("PATH TRAVERSAL SUCCESS: /etc/passwd exposed!")
        else:
            log_ok("Path traversal attempt blocked")

        # Random filename test
        name = random_filename()
        r = requests.get(f"{BASE_URL}/files/{name}/meta")
        check_status(f"GET /files/{name}/meta", r)

        # DELETE normal
        r = requests.delete(f"{BASE_URL}/files/test.txt")
        check_status("DELETE /files/test.txt", r)

        # DELETE traversal attempt
        r = requests.delete(f"{BASE_URL}/files/../../etc/passwd")
        if r.status_code < 400:
            log_fail("DELETE path traversal may have succeeded!")
        else:
            log_ok("DELETE traversal blocked")

if __name__ == "__main__":
    test_routes()
