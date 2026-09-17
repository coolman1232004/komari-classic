"""Exercise the actual container over HTTP using disposable CI credentials."""
import json
import http.client
import subprocess
from pathlib import Path
import tempfile
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
# The server rejects an oversized Content-Length before reading the body.
# Read that response directly instead of uploading after the socket closes.
oversized = http.client.HTTPConnection('127.0.0.1', 25774, timeout=10)
oversized.putrequest('POST', '/api/login')
oversized.putheader('Content-Length', str(8*1024*1024+1))
oversized.endheaders()
assert oversized.getresponse().status == 413
oversized.close()

# Run the built agent against the built server with a disposable node token.
state = Path(tempfile.gettempdir()) / 'komari-smoke-node'
if state.exists():
    saved = request('/api/admin/client/'+state.read_text(), headers={'Cookie': cookie})
    assert saved.status == 200 and json.load(saved)['name'] == 'docker-ci-node', 'Node did not survive restart'
created = request('/api/admin/client/add', b'{"name":"docker-ci-node"}', {'Cookie': cookie, 'Content-Type': 'application/json'})
assert created.status == 200, 'Node creation failed'
node = json.load(created)
assert 'uuid' in node and 'token' in node
state.write_text(node['uuid'])
subprocess.run(['docker', 'run', '-d', '--name', 'komari-agent-smoke',
                '--network', 'container:komari-smoke', 'komari-agent:test',
                '-e', BASE, '-t', node['token'], '--disable-auto-update',
                '--disable-web-ssh'], check=True, stdout=subprocess.DEVNULL)
try:
    for attempt in range(60):
        with request('/api/admin/client/'+node['uuid'], headers={'Cookie': cookie}) as response:
            info = json.load(response)
        if info.get('cpu_name') or info.get('os'):
            break
        time.sleep(1)
    else:
        raise AssertionError('Agent did not report basic information')
finally:
    subprocess.run(['docker', 'rm', '-f', 'komari-agent-smoke'], check=False, stdout=subprocess.DEVNULL)
# Changing claimed proxy headers must not bypass the account/peer limiter.
for attempt in range(11):
    failed = request('/api/login', b'{"username":"ci-rate-test","password":"wrong"}',
                     {'Content-Type': 'application/json', 'X-Forwarded-For': f'192.0.2.{attempt}'})
    expected = 401 if attempt < 10 else 429
    assert failed.status == expected, f'Login limit: {failed.status} != {expected}'
    if expected == 429:
        assert failed.headers.get('Retry-After')
print('Docker smoke checks passed: UI, login, authorization, origin rejection, body limit, persistence, agent and login throttling')
