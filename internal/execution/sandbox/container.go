// Package sandbox contains the host isolation boundary. Never execute submissions
// through a host interpreter, including in development or runtime fallback paths.
package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	p "github.com/varun/lexor/pkg/pluginapi"
)

var ErrUnavailable = errors.New("sandbox unavailable")

type Container struct{ Runtime, Image string }

func (*Container) ID() string { return "container" }
func (c *Container) Available(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := exec.LookPath(c.Runtime); err != nil {
		return fmt.Errorf("%w: install Docker Desktop (macOS) or Docker Engine (Linux)", ErrUnavailable)
	}
	if b, err := exec.CommandContext(ctx, c.Runtime, "info", "--format", "{{.ServerVersion}}").CombinedOutput(); err != nil {
		return fmt.Errorf("%w: start your container runtime (%s)", ErrUnavailable, strings.TrimSpace(string(b)))
	}
	if err := exec.CommandContext(ctx, c.Runtime, "image", "inspect", c.Image).Run(); err != nil {
		return fmt.Errorf("%w: download the pinned runtime once with: %s pull %s", ErrUnavailable, c.Runtime, c.Image)
	}
	return nil
}

const bootstrap = `import sys,json,os
p=json.load(sys.stdin)
for name,contents in p['files'].items():
    if '/' in name or name in ('.','..'): raise ValueError('unsafe artifact')
    with open('/work/'+name,'w',encoding='utf-8') as f: f.write(contents)
os.execvp(p['command'][0],p['command'])
`

type cappedWriter struct {
	mu        sync.Mutex
	remaining int
	exceeded  bool
	cancel    context.CancelFunc
}
type streamWriter struct {
	cap *cappedWriter
	buf *bytes.Buffer
}

func (w streamWriter) Write(b []byte) (int, error) {
	w.cap.mu.Lock()
	defer w.cap.mu.Unlock()
	n := len(b)
	if n > w.cap.remaining {
		n = w.cap.remaining
		w.cap.exceeded = true
	}
	if n > 0 {
		w.buf.Write(b[:n])
		w.cap.remaining -= n
	}
	if w.cap.exceeded {
		w.cap.cancel()
		return len(b), nil
	}
	return len(b), nil
}
func (c *Container) Execute(ctx context.Context, a p.Artifact, l p.Limits) (p.SandboxResult, error) {
	var r p.SandboxResult
	files := map[string]string{}
	for name, b := range a.Files {
		if strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
			return r, fmt.Errorf("unsafe artifact name")
		}
		files[name] = string(b)
	}
	payload, err := json.Marshal(map[string]any{"files": files, "command": a.Command})
	if err != nil {
		return r, err
	}
	ctx, timeoutCancel := context.WithTimeout(ctx, l.Timeout)
	defer timeoutCancel()
	job, cancel := context.WithCancel(ctx)
	defer cancel()
	name := "lexor-" + uuid.NewString()
	memory := strconv.Itoa(l.MemoryMB) + "m"
	args := []string{"run", "--name", name, "--pull=never", "--network=none", "--read-only", "--user=65534:65534", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--pids-limit=32", "--memory=" + memory, "--memory-swap=" + memory, "--cpus=1", "--ulimit=nofile=64:64", "--ulimit=fsize=16777216:16777216", "--ulimit=cpu=" + strconv.Itoa(int(l.Timeout.Seconds())+1), "--ipc=none", "--tmpfs=/work:rw,noexec,nosuid,nodev,size=16777216,mode=1777", "--tmpfs=/tmp:rw,noexec,nosuid,nodev,size=8388608,mode=1777", "--workdir=/work", "-i", c.Image, "python", "-I", "-B", "-c", bootstrap}
	cmd := exec.CommandContext(job, c.Runtime, args...)
	cmd.Stdin = bytes.NewReader(payload)
	var out, stderr bytes.Buffer
	cap := &cappedWriter{remaining: l.OutputBytes, cancel: cancel}
	cmd.Stdout = streamWriter{cap, &out}
	cmd.Stderr = streamWriter{cap, &stderr}
	start := time.Now()
	err = cmd.Run()
	r.Duration = time.Since(start)
	r.Output = out.Bytes()
	r.Stderr = stderr.String()
	r.OutputExceeded = cap.exceeded
	r.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
	r.ExitCode = 0
	if err != nil {
		r.ExitCode = -1
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			r.ExitCode = ee.ExitCode()
		}
	}
	cleanup, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cleanCancel()
	// Inspect before removal so an OOM kill has a distinct result.
	if b, e := exec.CommandContext(cleanup, c.Runtime, "inspect", "--format", "{{.State.OOMKilled}}", name).Output(); e == nil {
		r.OOMKilled = strings.TrimSpace(string(b)) == "true"
	}
	removeErr := exec.CommandContext(cleanup, c.Runtime, "rm", "--force", name).Run()
	if removeErr != nil && ctx.Err() == nil {
		return r, fmt.Errorf("failed to clean sandbox container %s: %w", name, removeErr)
	}
	if err != nil && r.ExitCode == 125 {
		return r, fmt.Errorf("container runtime failed: %s", r.Stderr)
	}
	return r, nil
}
