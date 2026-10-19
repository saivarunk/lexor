//go:build integration

package execution_test

import (
	"context"
	"github.com/varun/lexor/internal/execution"
	"github.com/varun/lexor/internal/testutil"
	p "github.com/varun/lexor/pkg/pluginapi"
	"os"
	"strings"
	"testing"
	"time"
)

const correct = `def two_sum(nums, target):
    seen = {}
    for i, n in enumerate(nums):
        if target-n in seen:
            return [seen[target-n], i]
        seen[n] = i
`

func TestRealSandbox(t *testing.T) {
	if os.Getenv("LEXOR_DOCKER_TESTS") != "1" {
		t.Skip("set LEXOR_DOCKER_TESTS=1; requires a running Docker-compatible runtime and pinned image")
	}
	a := testutil.App(t)
	if err := a.Sandbox.Available(context.Background()); err != nil {
		t.Fatal(err)
	}
	limits := a.Engine.Limits
	limits.Timeout = 15 * time.Second
	e := execution.New(a.Library, a.Languages, a.Sandbox, a.Store, limits)
	for _, tc := range []struct{ name, source, status string }{
		{"accepted", correct, "Accepted"},
		{"wrong answer", "def two_sum(nums, target):\n    return [0,0]\n", "Wrong Answer"},
		{"runtime error", "def two_sum(nums, target):\n    raise ValueError('failure')\n", "Runtime Error"},
		{"syntax error", "def two_sum(\n", "Compilation Error"},
		{"output cap", "import os\ndef two_sum(nums, target):\n    os.write(1,b'x'*1000000)\n", "Output Limit Exceeded"},
		{"memory cap", "def two_sum(nums,target):\n    data = bytearray(1024*1024*1024)\n", "Memory Limit Exceeded"},
		{"non-finite result", "def two_sum(nums,target):\n    return float('nan')\n", "Runtime Error"},
		{"blocked network", "import socket\ntry:\n    socket.create_connection(('1.1.1.1',443),timeout=0.5)\nexcept OSError:\n    pass\nelse:\n    raise AssertionError('network unexpectedly accessible')\n" + correct, "Accepted"},
		{"PID cap", "import subprocess, errno\nchildren=[]\nblocked=False\ntry:\n    for i in range(64):\n        children.append(subprocess.Popen(['/bin/sleep','10']))\nexcept OSError as exc:\n    assert exc.errno == errno.EAGAIN\n    blocked=True\nfinally:\n    for child in children: child.terminate()\n    for child in children: child.wait()\nassert blocked, 'process creation was not limited'\n" + correct, "Accepted"},
		{"nonroot readonly and no host mounts", "import os\nassert os.getuid() != 0\nassert not os.path.exists('/var/run/docker.sock')\ntry:\n    open('/host-write-test','w')\nexcept OSError:\n    pass\nelse:\n    raise AssertionError('root filesystem writable')\n" + correct, "Accepted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := e.Execute(context.Background(), p.ExecutionRequest{ProblemID: "arrays.two-sum", Language: "python", Source: tc.source}, "submit")
			if err != nil || r.Status != tc.status {
				t.Fatalf("got %s want %s: %v %+v", r.Status, tc.status, err, r)
			}
			if tc.name == "accepted" && r.MemoryBytes <= 0 {
				t.Fatal("memory metric missing")
			}
			if strings.Contains(r.Stdout, "network unexpectedly") {
				t.Fatal("network restriction failed")
			}
		})
	}
	t.Run("job timeout", func(t *testing.T) {
		limits.Timeout = 2 * time.Second
		timeoutEngine := execution.New(a.Library, a.Languages, a.Sandbox, a.Store, limits)
		start := time.Now()
		r, err := timeoutEngine.Execute(context.Background(), p.ExecutionRequest{ProblemID: "arrays.two-sum", Language: "python", Source: "def two_sum(nums,target):\n    while True: pass\n"}, "run")
		if err != nil || r.Status != "Time Limit Exceeded" || time.Since(start) > 9*time.Second {
			t.Fatalf("timeout failed: %+v %v", r, err)
		}
	})
}
