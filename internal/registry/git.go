package registry

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// Git's HTTPS traffic passes through this CONNECT proxy so each outbound socket
// gets the same DNS/IP policy as JSON imports. The TLS tunnel is end-to-end Git ↔
// origin. Clone hooks, global config, alternate protocols and redirects are off.
func (r *Resolver) resolveRemoteGit(ctx context.Context, ref string) (Package, error) {
	u, err := validateURL(ref)
	if err != nil {
		return Package{}, err
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return Package{}, fmt.Errorf("Git URLs cannot contain query strings or fragments")
	}
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	directory, err := os.MkdirTemp("", "lexor-git-")
	if err != nil {
		return Package{}, err
	}
	defer os.RemoveAll(directory)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Package{}, err
	}
	var transferred atomic.Int64
	proxy := &http.Server{ReadHeaderTimeout: 3 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != "CONNECT" {
			http.Error(w, "CONNECT required", 405)
			return
		}
		host, port, e := net.SplitHostPort(req.Host)
		if e != nil || port != "443" {
			http.Error(w, "HTTPS port 443 required", 403)
			return
		}
		ips, e := net.DefaultResolver.LookupHost(ctx, host)
		if e != nil || len(ips) == 0 {
			http.Error(w, "DNS lookup failed", 502)
			return
		}
		for _, ip := range ips {
			if !publicIP(ip) {
				http.Error(w, "private/reserved networks blocked", 403)
				return
			}
		}
		upstream, e := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(ips[0], port))
		if e != nil {
			http.Error(w, "upstream failed", 502)
			return
		}
		defer upstream.Close()
		deadline := time.Now().Add(40 * time.Second)
		upstream.SetDeadline(deadline)
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		client, buffer, e := hijacker.Hijack()
		if e != nil {
			return
		}
		defer client.Close()
		client.SetDeadline(deadline)
		if _, e = buffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); e != nil {
			return
		}
		if buffer.Flush() != nil {
			return
		}
		go func() { _, _ = io.Copy(upstream, buffer); upstream.Close() }()
		_, _ = io.Copy(budgetWriter{client, &transferred}, upstream)
	})}
	go proxy.Serve(listener)
	defer proxy.Close()
	repo := filepath.Join(directory, "repo.git")
	proxyURL := "http://" + listener.Addr().String()
	args := []string{"-c", "core.hooksPath=/dev/null", "-c", "init.templateDir=", "-c", "http.proxy=" + proxyURL, "-c", "http.followRedirects=false", "-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "clone", "--bare", "--depth=1", "--", ref, repo}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = gitEnvironment()
	var log limitedBuffer
	cmd.Stdout = &log
	cmd.Stderr = &log
	if err = cmd.Run(); err != nil {
		return Package{}, fmt.Errorf("safe Git clone failed: %s: %w", log.String(), err)
	}
	archive := exec.CommandContext(ctx, "git", "-C", repo, "-c", "core.hooksPath=/dev/null", "archive", "--format=tar", "HEAD")
	archive.Env = gitEnvironment()
	pipe, err := archive.StdoutPipe()
	if err != nil {
		return Package{}, err
	}
	archive.Stderr = &log
	if err = archive.Start(); err != nil {
		return Package{}, err
	}
	unpack := filepath.Join(directory, "problems")
	if err = os.Mkdir(unpack, 0700); err != nil {
		return Package{}, err
	}
	reader := tar.NewReader(io.LimitReader(pipe, (20<<20)+1))
	total := int64(0)
	files := 0
	for {
		header, e := reader.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			err = e
			break
		}
		if header.Typeflag == tar.TypeXGlobalHeader || header.Typeflag == tar.TypeXHeader {
			continue
		}
		name := strings.TrimSuffix(header.Name, "/")
		if name == "" {
			continue
		}
		if !filepath.IsLocal(name) || strings.Contains(name, "\\") {
			err = fmt.Errorf("unsafe Git archive path")
			break
		}
		dest := filepath.Join(unpack, filepath.FromSlash(name))
		if header.Typeflag == tar.TypeDir {
			err = os.MkdirAll(dest, 0700)
			if err != nil {
				break
			}
			continue
		}
		if header.Typeflag != tar.TypeReg {
			err = fmt.Errorf("Git archive symlinks and special files are blocked")
			break
		}
		total += header.Size
		files++
		if total > 16<<20 || header.Size > maxBytes || files > 2000 {
			err = fmt.Errorf("Git archive exceeds limits (16 MB, 2 MB per file, 2000 files)")
			break
		}
		if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			break
		}
		f, e := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			err = e
			break
		}
		_, err = io.CopyN(f, reader, header.Size)
		f.Close()
		if err != nil {
			break
		}
	}
	if err != nil {
		archive.Process.Kill()
		archive.Wait()
		return Package{}, err
	}
	if err = archive.Wait(); err != nil {
		return Package{}, fmt.Errorf("Git archive: %w", err)
	}
	pack, err := r.Resolve(ctx, unpack)
	pack.Source = ref
	return pack, err
}

type budgetWriter struct {
	io.Writer
	total *atomic.Int64
}

func (w budgetWriter) Write(b []byte) (int, error) {
	if w.total.Add(int64(len(b))) > 32<<20 {
		return 0, fmt.Errorf("Git download exceeds 32 MB")
	}
	return w.Writer.Write(b)
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.Len() < 8192 {
		remaining := 8192 - b.Len()
		if len(p) > remaining {
			p = p[:remaining]
		}
		b.Buffer.Write(p)
	}
	return n, nil
}
func gitEnvironment() []string {
	env := []string{}
	for _, entry := range os.Environ() {
		key := strings.SplitN(entry, "=", 2)[0]
		upper := strings.ToUpper(key)
		if strings.HasPrefix(upper, "GIT_") || strings.Contains(upper, "PROXY") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ALLOW_PROTOCOL=https")
}
