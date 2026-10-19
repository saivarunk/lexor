"""Container-only supervisor: user OS stdout is never the result protocol."""
import json
import os
import selectors
import signal
import subprocess
import sys

with open('/work/request.json', encoding='utf-8') as f:
    limit = json.load(f)['outputLimit']
child = subprocess.Popen(['python', '-I', '-B', '/work/harness.py'], stdout=subprocess.PIPE,
                         stderr=subprocess.PIPE, start_new_session=True)
selector = selectors.DefaultSelector()
selector.register(child.stdout, selectors.EVENT_READ, 'stdout')
selector.register(child.stderr, selectors.EVENT_READ, 'stderr')
streams = {'stdout': bytearray(), 'stderr': bytearray()}
size = 0
exceeded = False
while selector.get_map():
    for key, _ in selector.select():
        chunk = os.read(key.fileobj.fileno(), 8192)
        if not chunk:
            selector.unregister(key.fileobj)
            continue
        size += len(chunk)
        if size > limit:
            exceeded = True
            os.killpg(child.pid, signal.SIGKILL)
            selector.close()
            break
        streams[key.data].extend(chunk)
    if exceeded:
        break
code = child.wait()
if exceeded:
    result = {'actual': None, 'kind': 'Output Limit Exceeded', 'error': 'Output limit exceeded'}
elif code != 0:
    result = {'actual': None, 'kind': 'Runtime Error', 'error': 'Solution process exited with status ' + str(code)}
else:
    try:
        with open('/work/result.json', encoding='utf-8') as f:
            text = f.read(limit + 1)
        if len(text.encode('utf-8')) > limit:
            raise ValueError('Oversized result')
        result = json.loads(text)
        if not isinstance(result, dict):
            raise ValueError('Result must be an object')
    except (OSError, ValueError) as exc:
        result = {'actual': None, 'kind': 'Runtime Error', 'error': 'Missing or invalid structured result: ' + str(exc)}
for stream in streams:
    result[stream] = result.get(stream, '') + streams[stream].decode('utf-8', errors='replace')
sys.stdout.write(json.dumps(result, allow_nan=False))
