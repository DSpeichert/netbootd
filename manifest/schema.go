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

// normalizePath strips leading and trailing slashes to produce a canonical
// path for comparison. This ensures "/foo/bar", "foo/bar", and "foo/bar/"
// are all treated identically.
func normalizePath(p string) string {
	return strings.Trim(p, "/")
}

// pathHasPrefix reports whether the normalized request path starts with the
// normalized mount prefix at a path-segment boundary. An empty prefix matches
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
		suffix = m.PathSuffix(requestPath)
	}
	return filepath.Join(m.hostPathPrefix(rootPath), suffix)
}

func (m Mount) ValidateHostPath(rootPath string, hostPath string) bool {
	return strings.HasPrefix(hostPath, m.hostPathPrefix(rootPath))
}

// PathSuffix extracts the portion of requestPath that extends beyond the
// mount's Path. Both paths are normalized before comparison so that
// leading/trailing slashes do not affect the result. The returned suffix
// always starts with "/" (or is empty when the paths are equal).
// PathSuffix enforces segment-boundary matching: mount "foo" will not match
// request "foobar".
func (m Mount) PathSuffix(requestPath string) string {
	nReq := normalizePath(requestPath)
	nMount := normalizePath(m.Path)
	if nMount == "" {
		if nReq == "" {
			return ""
		}
		return "/" + nReq
	}
	if !pathHasPrefix(nReq, nMount) {
		// no segment-boundary prefix match — return the full request as-is
		return "/" + nReq
	}
	rest := strings.TrimPrefix(nReq, nMount)
	if rest == "" {
		return ""
	}
	// rest always starts with "/" because pathHasPrefix guarantees segment-boundary matching
	return rest
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
			suffix := m.PathSuffix(req.URL.Path)
			req.URL.Path = path.Join(target.Path, suffix)
			if req.URL.RawPath != "" {
				rawSuffix := m.PathSuffix(req.URL.RawPath)
				req.URL.RawPath = path.Join(target.RawPath, rawSuffix)
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
// Both the request path and mount paths are normalized (leading/trailing slashes
// stripped) before comparison. Prefix matches are checked at path-segment
// boundaries so that mount "foo" does not match request "foobar".
func (m *Manifest) GetMount(reqPath string) (Mount, error) {
	nPath := normalizePath(reqPath)
	var bestMount Mount
	var bestLen int
	var found bool
	for _, mount := range m.Mounts {
		nMountPath := normalizePath(mount.Path)
		if !mount.PathIsPrefix && nMountPath == nPath {
			return mount, nil
		} else if mount.PathIsPrefix &&
			pathHasPrefix(nPath, nMountPath) &&
			(len(nMountPath) > bestLen || !found) {
			bestMount = mount
			bestLen = len(nMountPath)
			found = true
		}
	}

	if found {
		return bestMount, nil
	}
	return bestMount, errors.New("no mount matches path: " + reqPath)
}
