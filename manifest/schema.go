package manifest

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// cleanPath resolves p into its canonical form for path matching and
// joining: "." and ".." segments are resolved as if p were rooted (so ".."
// can never climb above p's own root), repeated slashes collapse, and the
// leading and trailing slashes are stripped. hadTrailingSlash reports
// whether the non-empty cleaned path represents a directory-style request
// (the input ended in "/"), which matters when the suffix is later appended
// to a proxied URL or local path.
func cleanPath(p string) (cleaned string, hadTrailingSlash bool) {
	trailing := len(p) > 1 && strings.HasSuffix(p, "/")
	c := strings.TrimPrefix(path.Clean("/"+p), "/")
	if c == "." {
		c = ""
	}
	return c, trailing && c != ""
}

// pathHasPrefix reports whether the cleaned request path starts with the
// cleaned mount prefix at a path-segment boundary. An empty prefix matches
// everything. This prevents "foo" from incorrectly matching "foobar".
func pathHasPrefix(requestPath, prefix string) bool {
	if prefix == "" {
		return true
	}
	if requestPath == prefix {
		return true
	}
	// The request must continue with a "/" after the prefix.
	return strings.HasPrefix(requestPath, prefix+"/")
}

// EscapePathSuffix percent-escapes each segment of a canonical, decoded path
// suffix (as returned by Mount.PathSuffix) so it can be safely appended to a
// proxy target URL via (*url.URL).JoinPath. JoinPath treats its arguments as
// already-escaped URL syntax, so passing a raw suffix through unescaped risks
// either corrupting bytes that happen to look like percent-escapes or, if
// they are invalid escapes, having the whole suffix silently dropped.
func EscapePathSuffix(suffix string) string {
	if suffix == "" {
		return ""
	}
	trailingSlash := strings.HasSuffix(suffix, "/")
	segments := strings.Split(strings.Trim(suffix, "/"), "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	out := "/" + strings.Join(segments, "/")
	if trailingSlash && out != "/" {
		out += "/"
	}
	return out
}

// Manifest represents user-supplied per-host manifest information.
// go-yaml accepts completely lowercase version of keys but is not case-insensitive
// https://github.com/go-yaml/yaml/issues/123
// some fields are forcefully mapped to camelCase instead of CamelCase and camelcase
type Manifest struct {
	ID            string        `yaml:"id"`
	IPv4          IPWithNet     `yaml:"ipv4"`
	Hostname      string        `yaml:"hostname"`
	Domain        string        `yaml:"domain"`
	LeaseDuration time.Duration `yaml:"leaseDuration"`
	MAC           []HardwareAddr
	DNS           []net.IP
	Router        []net.IP
	NTP           []net.IP
	Ipxe          bool
	BootFilename  string `yaml:"bootFilename"`
	Mounts        []Mount
	Suspended     bool
	Vars          map[string]interface{}
}

// Mount represents a path exposed via TFTP and HTTP.
type Mount struct {
	// Path at which to select this mount.
	Path string

	// If Prefix is set to true, the Path is treated as a prefix.
	PathIsPrefix bool `yaml:"pathIsPrefix"`

	// The proxy destination used when handling requests.
	// Mutually exclusive with Content option.
	Proxy string
	// If PathIsPrefix is true and AppendSuffix is true, the suffix to Path Prefix will also be appended to Proxy Or LocalDir.
	// Otherwise, it will be many to one proxy.
	AppendSuffix bool `yaml:"appendSuffix"`

	// Provides content template (passed through template/text) to serve.
	// Mutually exclusive with Proxy option.
	Content string

	// Provides a path on the host to find the files.
	// So that LocalDir: /tftpboot path: /subdir and client requests: /subdir/file.x the path on the host
	// becomes /tfptboot/file.x
	// If LocalDir is not absolute, path is relative to rootPath passed into HostPath and ValidateHostPath.
	// So that RootPath: /tftpboot, LocalDir: ./files, path: /subdir and client request: /subdir/file.x on the host
	// becomes /tftpboot/files/file.x
	LocalDir string `yaml:"localDir"`
}

func (m Mount) hostPathPrefix(rootPath string) string {
	if filepath.IsAbs(m.LocalDir) {
		return m.LocalDir
	}
	return filepath.Join(rootPath, m.LocalDir)
}

func (m Mount) HostPath(rootPath, requestPath string) string {
	suffix := m.Path
	if m.AppendSuffix {
		if s, ok := m.PathSuffix(requestPath); ok {
			suffix = s
		} else {
			suffix = ""
		}
	}
	return filepath.Join(m.hostPathPrefix(rootPath), suffix)
}

func (m Mount) ValidateHostPath(rootPath string, hostPath string) bool {
	base := filepath.Clean(m.hostPathPrefix(rootPath))
	target := filepath.Clean(hostPath)
	rel, err := filepath.Rel(base, target)
	return err == nil && filepath.IsLocal(rel)
}

// PathSuffix extracts the canonical portion of requestPath that extends
// beyond the mount's Path, for callers that append it to a proxy target or
// local directory (AppendSuffix). Both paths are cleaned via cleanPath
// before comparison, using the same segment-boundary matching as GetMount,
// so a suffix is only ever extracted from a request that GetMount would
// have routed to this mount. ok is false when requestPath does not fall
// under the mount's Path at a segment boundary; this should never happen
// for a mount already selected by GetMount, since both use identical
// matching rules. A trailing slash on requestPath is preserved on the
// returned suffix so directory-style proxy requests are not truncated.
func (m Mount) PathSuffix(requestPath string) (suffix string, ok bool) {
	nReq, trailingSlash := cleanPath(requestPath)
	nMount, _ := cleanPath(m.Path)

	if !pathHasPrefix(nReq, nMount) {
		return "", false
	}

	rest := strings.TrimPrefix(strings.TrimPrefix(nReq, nMount), "/")
	if rest == "" {
		if trailingSlash {
			return "/", true
		}
		return "", true
	}
	if trailingSlash {
		rest += "/"
	}
	return "/" + rest, true
}

func (m Mount) ProxyDirector() (func(req *http.Request), error) {
	target, err := url.Parse(m.Proxy)
	if err != nil {
		return nil, err
	}

	// we're not removing the possible "spoof" query param
	director := func(req *http.Request) {
		//requestDump, err := httputil.DumpRequest(req, true)
		//if err != nil {
		//	fmt.Println(err)
		//}
		//fmt.Println("original request: " + string(requestDump))

		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host

		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}

		if m.AppendSuffix {
			suffix, ok := m.PathSuffix(req.URL.Path)
			if !ok || suffix == "" {
				// A request for the mount's own root (or one that
				// couldn't be matched, which shouldn't happen for a
				// mount GetMount already selected) maps to the
				// configured target verbatim. Going through JoinPath
				// with an empty suffix would silently drop a trailing
				// slash the operator configured on the target.
				req.URL.Path = target.Path
				req.URL.RawPath = target.RawPath
			} else {
				joined := target.JoinPath(EscapePathSuffix(suffix))
				req.URL.Path = joined.Path
				req.URL.RawPath = joined.RawPath
			}
		} else {
			req.URL.Path = target.Path
			req.URL.RawPath = target.RawPath
		}

		//requestDump, err = httputil.DumpRequest(req, true)
		//if err != nil {
		//	fmt.Println(err)
		//}
		//fmt.Println("modified request: " + string(requestDump))
	}

	return director, nil
}

// ContentContext is the template context available for static Content embedded in Manifests.
type ContentContext struct {
	// Address of netbootd server
	LocalIP net.IP
	// Address of client
	RemoteIP net.IP
	// Base URL to the HTTP service (IP and port) - not API
	HttpBaseUrl *url.URL
	// Base URL to the API service (IP and port)
	ApiBaseUrl *url.URL
	// Host to Syslog service (IP and port)
	SyslogHost string
	// Copy of Manifest
	Manifest *Manifest
}

// GetMount returns best matching Mount, respecting exact and prefix-based mount paths.
// Longest path match is considered "best".
// Both the request path and mount paths are cleaned (via cleanPath) before
// comparison, so leading/trailing slashes and "." / ".." segments do not
// affect matching. Prefix matches are checked at path-segment boundaries so
// that mount "foo" does not match request "foobar".
func (m *Manifest) GetMount(reqPath string) (Mount, error) {
	nPath, _ := cleanPath(reqPath)
	var bestMount Mount
	bestLen := -1
	for _, mount := range m.Mounts {
		nMountPath, _ := cleanPath(mount.Path)
		if !mount.PathIsPrefix && nMountPath == nPath {
			return mount, nil
		} else if mount.PathIsPrefix &&
			pathHasPrefix(nPath, nMountPath) &&
			len(nMountPath) > bestLen {
			bestMount = mount
			bestLen = len(nMountPath)
		}
	}

	if bestLen >= 0 {
		return bestMount, nil
	}
	return bestMount, errors.New("no mount matches path: " + reqPath)
}
