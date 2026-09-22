"""Round-trip the actual container's backup format using disposable CI data."""
import io
import json
import subprocess
import time
import urllib.error
import urllib.request
import zipfile

BASE = "http://127.0.0.1:25774"


def request(path, body=None, headers=None):
    try:
        return urllib.request.urlopen(urllib.request.Request(BASE + path, data=body, headers=headers or {}), timeout=60)
    except urllib.error.HTTPError as error:
        return error


def login():
    response = request("/api/login", json.dumps({"username": "ci-admin", "password": "ci-test-password"}).encode(), {"Content-Type": "application/json"})
    assert response.status == 200, f"Login failed: {response.status}"
    return response.headers["Set-Cookie"].split(";", 1)[0]


def upload(payload, cookie):
    boundary = "komari-ci-backup-boundary"
    body = (f'--{boundary}\r\nContent-Disposition: form-data; name="backup"; filename="backup.zip"\r\nContent-Type: application/zip\r\n\r\n'.encode()
            + payload + f"\r\n--{boundary}--\r\n".encode())
    return request("/api/admin/upload/backup", body, {"Cookie": cookie, "Content-Type": f"multipart/form-data; boundary={boundary}"})


cookie = login()
headers = {"Cookie": cookie}
before = request("/api/admin/client/list", headers=headers).read()
response = request("/api/admin/download/backup", headers=headers)
assert response.status == 200, response.read().decode(errors="replace")
archive = response.read()
with zipfile.ZipFile(io.BytesIO(archive)) as z:
    manifest = json.loads(z.read("komari-backup.json"))
    assert manifest["format"] == 1 and "komari.db" in manifest["files"]
    assert not any(".komari-restore" in name for name in z.namelist())
    corrupt = io.BytesIO()
    with zipfile.ZipFile(corrupt, "w") as out:
        for name in z.namelist():
            out.writestr(name, b"invalid database" if name == "komari.db" else z.read(name))
assert upload(corrupt.getvalue(), cookie).status == 400, "Corrupt backup accepted"
legacy = io.BytesIO()
with zipfile.ZipFile(legacy, "w") as z:
    z.writestr("komari-backup-markup", "legacy backup")
assert upload(legacy.getvalue(), cookie).status == 400, "Unversioned backup accepted"
assert request("/api/admin/client/list", headers=headers).read() == before, "Rejected upload changed live data"
assert upload(archive, cookie).status == 200, "Valid backup was not queued"
assert upload(archive, cookie).status == 409, "Pending backup could be overwritten"
time.sleep(3)
for attempt in range(60):
    # compose run may not carry a restart policy on every Docker version.
    state = subprocess.check_output(["docker", "inspect", "-f", "{{.State.Status}}", "komari-smoke"], text=True).strip()
    if state == "exited":
        subprocess.run(["docker", "start", "komari-smoke"], check=True, stdout=subprocess.DEVNULL)
    try:
        if request("/ping").status == 200:
            break
    except OSError:
        pass
    time.sleep(1)
else:
    raise AssertionError("Container did not recover after backup restoration")
assert request("/api/admin/client/list", headers=headers).status == 401, "Restored browser session remained valid"
cookie = login()
after = request("/api/admin/client/list", headers={"Cookie": cookie}).read()
assert json.loads(after) == json.loads(before), "Client data changed during backup round-trip"
assert request("/api/admin/download/backup", headers={"Cookie": cookie}).status == 200, "Re-export failed after restore"
subprocess.run(["docker", "exec", "komari-smoke", "sh", "-c", "test ! -e /app/data/.restore-in-progress && test ! -e /app/data/backup.zip && test -n \"$(find /app/data/.komari-restore -name source.zip -print -quit)\""], check=True)
print("Docker backup checks passed: export, corruption/legacy rejection, exclusive queue, restart, session revocation, data preservation and persistent recovery files")
