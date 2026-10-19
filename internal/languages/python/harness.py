"""Runs only inside Lexor's sandbox. Inputs are JSON, never Python expressions.

Each test gets a fresh container. Results go to a dedicated file; the supervisor
captures user OS stdout/stderr separately. Container status determines failures.
This is a grading protocol, not a security boundary against result forgery by the
owner of this local workbench. The container is the host isolation boundary.
"""
import contextlib
import io
import json
import traceback
import sys
import resource

protocol = open('/work/result.json', 'w', encoding='utf-8')
with open('/work/request.json', encoding='utf-8') as f:
    request = json.load(f)

class BoundedText(io.TextIOBase):
    def __init__(self, limit):
        self.parts = []
        self.length = 0
        self.limit = limit
    def write(self, text):
        n = len(text.encode('utf-8'))
        if self.length + n > self.limit:
            raise RuntimeError('Output limit exceeded')
        self.parts.append(text)
        self.length += n
        return len(text)
    def value(self):
        return ''.join(self.parts)

out = BoundedText(request['outputLimit'])
err = BoundedText(request['outputLimit'])
response = {'actual': None, 'stdout': '', 'stderr': '', 'error': '', 'kind': ''}
try:
    with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
        namespace = {'__name__': '__lexor_solution__'}
        with open('/work/solution.py', encoding='utf-8') as f:
            source = f.read()
        code = compile(source, 'solution.py', 'exec')
        exec(code, namespace)
        fn = namespace[request['entrypoint']['name']]
        args = [request['input'][p['name']] for p in request['entrypoint']['parameters']]
        response['actual'] = fn(*args)
        json.dumps(response['actual'], allow_nan=False)
except BaseException as exc:
    response['kind'] = 'Compilation Error' if isinstance(exc, SyntaxError) else 'Output Limit Exceeded' if str(exc) == 'Output limit exceeded' else 'Runtime Error'
    response['error'] = ''.join(traceback.format_exception_only(type(exc), exc))[:4096]
response['memoryBytes'] = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss * 1024
response['stdout'] = out.value()
response['stderr'] = err.value()
try:
    protocol.write(json.dumps(response, allow_nan=False))
except (TypeError, ValueError):
    protocol.write(json.dumps({'actual': None, 'stdout': '', 'stderr': '', 'kind': 'Runtime Error', 'error': 'Return value is not finite JSON'}))
