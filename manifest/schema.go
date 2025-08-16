package manifest

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

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
	path := m.Path
	if m.AppendSuffix {
		path = strings.TrimPrefix(requestPath, m.Path)
	}
	return filepath.Join(m.hostPathPrefix(rootPath), path)
}

func (m Mount) ValidateHostPath(rootPath string, hostPath string) bool {
	return strings.HasPrefix(hostPath, m.hostPathPrefix(rootPath))
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
			req.URL.Path = target.Path + strings.TrimPrefix(req.URL.Path, m.Path)
			req.URL.RawPath = target.RawPath + strings.TrimPrefix(req.URL.RawPath, m.Path)
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
	// Copy of Manifest
	Manifest *Manifest
}

// GetMount returns best matching Mount, respecting exact and prefix-based mount paths.
// Longest path match is considered "best".
// If the path in the Mount or being matched begins with a slash (/), it is ignored.
func (m *Manifest) GetMount(path string) (Mount, error) {
	path = strings.TrimLeft(path, "/")
	var bestMount Mount
	var found bool
	for _, mount := range m.Mounts {
		mountPath := strings.TrimLeft(mount.Path, "/")
		if !mount.PathIsPrefix && mountPath == path {
			return mount, nil
		} else if mount.PathIsPrefix &&
			(mountPath == "" || strings.HasPrefix(path, mountPath)) &&
			(len(mount.Path) > len(bestMount.Path) || !found) {
			bestMount = mount
			found = true
		}
	}

	if found {
		return bestMount, nil
	}
	return bestMount, errors.New("no mount matches path: " + path)
}
