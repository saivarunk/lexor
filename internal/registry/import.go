// Package registry resolves data-only problem packages. Remote references are
// explicit actions, validated at connection time, and never execute repository code.
package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/varun/lexor/internal/problems"
	p "github.com/varun/lexor/pkg/pluginapi"
)

const maxBytes = 2 << 20
const maxProblems = 200

type Package struct {
	Source   string      `json:"source"`
	Digest   string      `json:"digest"`
	Problems []p.Problem `json:"problems"`
}
type ProblemSource interface {
	Resolve(context.Context, string) (Package, error)
}
type Resolver struct {
	Library *problems.Library
	mu      sync.Mutex
}

func publicIP(addr string) bool {
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return false
	}
	for _, cidr := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		if netip.MustParsePrefix(cidr).Contains(ip) {
			return false
		}
	}
	return true
}
func validateURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return nil, fmt.Errorf("remote imports require HTTPS without credentials on port 443")
	}
	return u, nil
}
func PublicClient() *http.Client {
	tr := &http.Transport{Proxy: nil, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupHost(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no addresses")
		}
		for _, ip := range ips {
			if !publicIP(ip) {
				return nil, fmt.Errorf("private/reserved network imports are blocked")
			}
		}
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0], port))
	}}
	return &http.Client{Transport: tr, Timeout: 20 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		_, err := validateURL(req.URL.String())
		return err
	}}
}
func (r *Resolver) Resolve(ctx context.Context, ref string) (Package, error) {
	pack := Package{Source: ref, Problems: []p.Problem{}}
	if strings.HasPrefix(ref, "https://") {
		if _, err := validateURL(ref); err != nil {
			return pack, err
		}
		req, err := http.NewRequestWithContext(ctx, "GET", ref, nil)
		if err != nil {
			return pack, err
		}
		res, err := PublicClient().Do(req)
		if err != nil {
			var urlErr *url.Error
			if errors.As(err, &urlErr) {
				return pack, fmt.Errorf("HTTPS source failed: %w", urlErr.Err)
			}
			return pack, err
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return pack, fmt.Errorf("source returned HTTP %d", res.StatusCode)
		}
		b, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
		if err != nil {
			return pack, err
		}
		return r.decode(pack, b)
	}
	if strings.Contains(ref, "://") {
		return pack, fmt.Errorf("unsupported source scheme; use HTTPS or a local path")
	}
	st, err := os.Lstat(ref)
	if err != nil {
		return pack, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return pack, fmt.Errorf("symlink sources are blocked")
	}
	if !st.IsDir() {
		b, err := readBounded(ref)
		if err != nil {
			return pack, err
		}
		return r.decode(pack, b)
	}
	root, err := os.OpenRoot(ref)
	if err != nil {
		return pack, err
	}
	defer root.Close()
	// A manifest restricts discovery to its explicit, portable relative file list.
	var files []string
	if b, e := fs.ReadFile(root.FS(), "lexor-pack.json"); e == nil {
		var m struct {
			SchemaVersion string   `json:"schemaVersion"`
			Name          string   `json:"name"`
			Version       string   `json:"version"`
			Problems      []string `json:"problems"`
		}
		if e = json.Unmarshal(b, &m); e != nil {
			return pack, e
		}
		if m.SchemaVersion != "1.0.0" {
			return pack, fmt.Errorf("unsupported pack schema version")
		}
		files = m.Problems
	} else if !os.IsNotExist(e) {
		return pack, e
	} else {
		err = fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink in imported package: %s", path)
			}
			if d.IsDir() {
				if d.Name() == ".git" {
					return fs.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, ".json") {
				files = append(files, path)
				if len(files) > maxProblems {
					return fmt.Errorf("package exceeds %d problems", maxProblems)
				}
			}
			return nil
		})
		if err != nil {
			return pack, err
		}
	}
	if len(files) == 0 || len(files) > maxProblems {
		return pack, fmt.Errorf("package must contain 1–%d problems", maxProblems)
	}
	total := 0
	for _, file := range files {
		if !fs.ValidPath(file) || !strings.HasSuffix(file, ".json") {
			return pack, fmt.Errorf("unsafe manifest path %q", file)
		}
		st, e := root.Lstat(file)
		if e != nil {
			return pack, e
		}
		if !st.Mode().IsRegular() {
			return pack, fmt.Errorf("manifest entries must be regular files")
		}
		f, e := root.Open(file)
		if e != nil {
			return pack, e
		}
		b, e := io.ReadAll(io.LimitReader(f, maxBytes+1))
		f.Close()
		if e != nil {
			return pack, e
		}
		total += len(b)
		if total > 16<<20 {
			return pack, fmt.Errorf("package exceeds 16 MB")
		}
		part, e := r.decode(Package{Source: ref}, b)
		if e != nil {
			return pack, fmt.Errorf("%s: %w", file, e)
		}
		pack.Problems = append(pack.Problems, part.Problems...)
	}
	return finalize(pack)
}
func readBounded(path string) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxBytes+1))
}
func (r *Resolver) decode(pack Package, b []byte) (Package, error) {
	if len(b) > maxBytes {
		return pack, fmt.Errorf("JSON exceeds 2 MB")
	}
	var raws []json.RawMessage
	if len(strings.TrimSpace(string(b))) == 0 {
		return pack, fmt.Errorf("empty source")
	}
	if strings.TrimSpace(string(b))[0] == '[' {
		if err := json.Unmarshal(b, &raws); err != nil {
			return pack, err
		}
	} else {
		raws = []json.RawMessage{b}
	}
	if len(raws) > maxProblems {
		return pack, fmt.Errorf("too many problems")
	}
	for _, raw := range raws {
		pr, err := r.Library.Validate(raw)
		if err != nil {
			return pack, err
		}
		pack.Problems = append(pack.Problems, pr)
	}
	return finalize(pack)
}
func finalize(pack Package) (Package, error) {
	seen := map[string]bool{}
	for _, pr := range pack.Problems {
		if seen[pr.ID] {
			return pack, fmt.Errorf("duplicate ID %s in source", pr.ID)
		}
		seen[pr.ID] = true
	}
	b, _ := json.Marshal(pack.Problems)
	h := sha256.Sum256(b)
	pack.Digest = hex.EncodeToString(h[:])
	return pack, nil
}
func (r *Resolver) Install(pack Package) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	checked, err := finalize(pack)
	if err != nil {
		return nil, err
	}
	if len(checked.Problems) == 0 {
		return nil, fmt.Errorf("no problems")
	}
	for _, pr := range checked.Problems {
		b, _ := json.Marshal(pr)
		if _, err = r.Library.Validate(b); err != nil {
			return nil, err
		}
		if _, e := r.Library.Get(pr.ID); e == nil {
			return nil, fmt.Errorf("collision: %s already exists; imports never overwrite local data", pr.ID)
		}
	}
	installed := []string{}
	saved := []p.ProblemView{}
	for _, pr := range checked.Problems {
		b, _ := json.Marshal(pr)
		view, e := r.Library.Save(b, "", "")
		if e != nil {
			for _, created := range saved {
				if rollbackErr := r.Library.Delete(created.ID, created.Revision); rollbackErr != nil {
					e = errors.Join(e, fmt.Errorf("rollback %s: %w", created.ID, rollbackErr))
				}
			}
			return nil, e
		}
		saved = append(saved, view)
		installed = append(installed, pr.ID)
	}
	return installed, nil
}

// ResolveGit imports a local clone or an HTTPS repository using the guarded Git transport.
func (r *Resolver) ResolveGit(ctx context.Context, ref string) (Package, error) {
	if strings.HasPrefix(ref, "https://") {
		return r.resolveRemoteGit(ctx, ref)
	}
	if strings.Contains(ref, "://") || strings.HasPrefix(ref, "-") {
		return Package{}, fmt.Errorf("Git sources require a local clone or HTTPS repository")
	}
	abs, err := filepath.Abs(ref)
	if err != nil {
		return Package{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", abs, "rev-parse", "--show-toplevel")
	b, err := cmd.Output()
	if err != nil {
		return Package{}, fmt.Errorf("not a local Git repository: %w", err)
	}
	canonical, e := filepath.EvalSymlinks(abs)
	if e != nil {
		return Package{}, e
	}
	if filepath.Clean(strings.TrimSpace(string(b))) != canonical {
		return Package{}, fmt.Errorf("provide the root of the Git clone")
	}
	pack, err := r.Resolve(ctx, abs)
	pack.Source = "git+file:" + abs
	return pack, err
}
