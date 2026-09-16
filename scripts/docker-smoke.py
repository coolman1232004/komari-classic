"""Exercise the actual container over HTTP using disposable CI credentials."""
import json
import time
import urllib.error
import urllib.request

BASE = 'http://127.0.0.1:25774'
for attempt in range(60):
    try:
        with urllib.request.urlopen(BASE + '/ping', timeout=2) as response:
            assert response.read() == b'pong'
        break
    except (OSError, AssertionError):
        time.sleep(1)
else:
    raise SystemExit('Container did not become ready')

with urllib.request.urlopen(BASE + '/', timeout=10) as response:
    assert b'<html' in response.read().lower(), 'Embedded frontend missing'

def request(path, body=None, headers=None):
    req = urllib.request.Request(BASE+path, data=body, headers=headers or {})
    try:
        return urllib.request.urlopen(req, timeout=10)
    except urllib.error.HTTPError as error:
        return error

assert request('/api/admin/client/list').status == 401
body = json.dumps({'username': 'ci-admin', 'password': 'ci-test-password'}).encode()
login = request('/api/login', body, {'Content-Type': 'application/json'})
assert login.status == 200, 'Login failed'
cookie = login.headers['Set-Cookie']
assert 'HttpOnly' in cookie and 'SameSite=Lax' in cookie
assert request('/api/admin/client/list', headers={'Cookie': cookie}).status == 200
assert request('/api/admin/settings/', b'{}', {'Cookie': cookie, 'Origin': 'https://untrusted.invalid', 'Content-Type': 'text/plain'}).status == 403
assert request('/api/login', b'x' * (8*1024*1024+1)).status == 413
print('Docker smoke checks passed: UI, login, authorization, origin rejection, body limit, persistent login')
