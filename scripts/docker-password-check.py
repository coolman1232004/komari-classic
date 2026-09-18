"""Check the disposable CI database, optionally seed a legacy hash while stopped."""
import base64
import hashlib
import sqlite3
import sys

with sqlite3.connect('data/komari.db') as db:
    row = db.execute('SELECT passwd FROM users WHERE username=?', ('ci-admin',)).fetchone()
    assert row and row[0].startswith('$argon2id$v=19$m=19456,t=2,p=1$'), 'Password was not stored/upgraded as Argon2id'
    if '--seed-legacy' in sys.argv:
        legacy = base64.b64encode(hashlib.sha256(b'ci-test-password06Wm4Jv1Hkxx').digest()).decode()
        db.execute('UPDATE users SET passwd=? WHERE username=?', (legacy, 'ci-admin'))
print('CI password storage verified')
