package manifest

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestCleanPath(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		want         string
		wantTrailing bool
	}{
		{"empty", "", "", false},
		{"slash only", "/", "", false},
		{"trailing slash", "foo/", "foo", true},
		{"leading slash", "/foo", "foo", false},
		{"both slashes", "/foo/", "foo", true},
		{"multiple leading", "///foo", "foo", false},
		{"nested", "/foo/bar/baz", "foo/bar/baz", false},
		{"nested trailing", "/foo/bar/", "foo/bar", true},
		{"no slashes", "foo/bar", "foo/bar", false},
		{"interior double slash", "/foo//bar", "foo/bar", false},
		{"dot segment", "/foo/./bar", "foo/bar", false},
		{"dot-dot climbs to root", "/foo/../bar", "bar", false},
		{"dot-dot cannot escape root", "/../../secret", "secret", false},
		{"dot-dot with trailing slash", "/foo/../bar/", "bar", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotTrailing := cleanPath(tt.in)
			if got != tt.want || gotTrailing != tt.wantTrailing {
				t.Errorf("cleanPath(%q) = (%q, %v), want (%q, %v)", tt.in, got, gotTrailing, tt.want, tt.wantTrailing)
			}
		})
	}
}

func TestPathHasPrefix(t *testing.T) {
	tests := []struct {
		name    string
		reqPath string
		prefix  string
		want    bool
	}{
		{"empty prefix matches anything", "foo/bar", "", true},
		{"exact match", "foo/bar", "foo/bar", true},
		{"proper prefix", "foo/bar/baz", "foo/bar", true},
		{"partial segment no match", "foobar", "foo", false},
		{"partial segment nested no match", "foo/barbaz", "foo/bar", false},
		{"prefix is longer", "foo", "foo/bar", false},
		{"empty request empty prefix", "", "", true},
		{"empty request nonempty prefix", "", "foo", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathHasPrefix(tt.reqPath, tt.prefix)
			if got != tt.want {
				t.Errorf("pathHasPrefix(%q, %q) = %v, want %v", tt.reqPath, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestGetMount_ExactMatch(t *testing.T) {
	m := &Manifest{
		Mounts: []Mount{
			{Path: "/foo/bar"},
			{Path: "baz"},
		},
	}

	tests := []struct {
		name    string
		reqPath string
		want    string
		wantErr bool
	}{
		{"with leading slash", "/foo/bar", "/foo/bar", false},
		{"without leading slash", "foo/bar", "/foo/bar", false},
		{"trailing slash", "/foo/bar/", "/foo/bar", false},
		{"no slashes", "baz", "baz", false},
		{"leading slash on config without", "/baz", "baz", false},
		{"no match", "/nonexistent", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mount, err := m.GetMount(tt.reqPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetMount(%q) error = %v, wantErr %v", tt.reqPath, err, tt.wantErr)
			}
			if !tt.wantErr && mount.Path != tt.want {
				t.Errorf("GetMount(%q).Path = %q, want %q", tt.reqPath, mount.Path, tt.want)
			}
		})
	}
}

func TestGetMount_PrefixMatch(t *testing.T) {
	m := &Manifest{
		Mounts: []Mount{
			{Path: "/images", PathIsPrefix: true},
			{Path: "/images/ubuntu", PathIsPrefix: true},
			{Path: "/", PathIsPrefix: true},
		},
	}

	tests := []struct {
		name    string
		reqPath string
		want    string
	}{
		{"root matches catch-all", "/somefile", "/"},
		{"images prefix", "/images/file.iso", "/images"},
		{"images/ubuntu more specific", "/images/ubuntu/file.iso", "/images/ubuntu"},
		{"images exact", "/images", "/images"},
		// Boundary check: "/imagesx" should NOT match "/images"
		{"no partial segment match", "/imagesx/file", "/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mount, err := m.GetMount(tt.reqPath)
			if err != nil {
				t.Fatalf("GetMount(%q) unexpected error: %v", tt.reqPath, err)
			}
			if mount.Path != tt.want {
				t.Errorf("GetMount(%q).Path = %q, want %q", tt.reqPath, mount.Path, tt.want)
			}
		})
	}
}

func TestGetMount_PrefixBoundary(t *testing.T) {
	// Ensure mount "foo" does NOT match request "foobar"
	m := &Manifest{
		Mounts: []Mount{
			{Path: "foo", PathIsPrefix: true},
		},
	}

	_, err := m.GetMount("foobar")
	if err == nil {
		t.Error("GetMount(\"foobar\") should not match mount \"foo\"")
	}

	// But "foo/bar" should match
	mount, err := m.GetMount("foo/bar")
	if err != nil {
		t.Fatalf("GetMount(\"foo/bar\") unexpected error: %v", err)
	}
	if mount.Path != "foo" {
		t.Errorf("GetMount(\"foo/bar\").Path = %q, want \"foo\"", mount.Path)
	}
}

func TestPathSuffix(t *testing.T) {
	tests := []struct {
		name      string
		mountPath string
		reqPath   string
		want      string
		wantOK    bool
	}{
		{"exact match", "/foo", "/foo", "", true},
		{"exact match no slashes", "foo", "foo", "", true},
		{"suffix extraction", "/foo", "/foo/bar", "/bar", true},
		{"mismatched leading slashes", "foo", "/foo/bar", "/bar", true},
		{"mount with slash request without", "/foo", "foo/bar/baz", "/bar/baz", true},
		{"trailing slash on mount", "foo/", "/foo/bar", "/bar", true},
		{"empty mount", "", "/foo/bar", "/foo/bar", true},
		{"empty mount empty request", "", "", "", true},
		{"deep suffix", "/a/b", "/a/b/c/d/e", "/c/d/e", true},
		{"partial segment no match", "foo", "foobar", "", false},
		{"partial segment nested no match", "foo/bar", "foo/barbaz", "", false},
		{"trailing slash preserved", "/foo", "/foo/bar/", "/bar/", true},
		{"trailing slash preserved at mount root", "/foo", "/foo/", "/", true},
		{"dot-dot cannot escape mount", "/foo", "/foo/../../secret", "", false},
		{"dot-dot within mount resolves", "/foo", "/foo/bar/../baz", "/baz", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Mount{Path: tt.mountPath}
			got, ok := m.PathSuffix(tt.reqPath)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("Mount{Path: %q}.PathSuffix(%q) = (%q, %v), want (%q, %v)",
					tt.mountPath, tt.reqPath, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestEscapePathSuffix(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"simple", "/bar", "/bar"},
		{"trailing slash", "/bar/", "/bar/"},
		{"root trailing slash", "/", "/"},
		{"space needs escaping", "/a b", "/a%20b"},
		{"literal percent is escaped, not misread", "/100%off", "/100%25off"},
		{"literal slash-looking escape stays literal", "/a%2Fb", "/a%252Fb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EscapePathSuffix(tt.in)
			if got != tt.want {
				t.Errorf("EscapePathSuffix(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestHostPath(t *testing.T) {
	tests := []struct {
		name        string
		mount       Mount
		rootPath    string
		requestPath string
		want        string
	}{
		{
			name:        "append suffix with slash mismatch",
			mount:       Mount{Path: "/subdir", AppendSuffix: true, LocalDir: "/tftpboot"},
			rootPath:    "/root",
			requestPath: "/subdir/file.x",
			want:        "/tftpboot/file.x",
		},
		{
			name:        "append suffix without leading slash",
			mount:       Mount{Path: "subdir", AppendSuffix: true, LocalDir: "/tftpboot"},
			rootPath:    "/root",
			requestPath: "/subdir/file.x",
			want:        "/tftpboot/file.x",
		},
		{
			name:        "no append suffix",
			mount:       Mount{Path: "/subdir", AppendSuffix: false, LocalDir: "/tftpboot"},
			rootPath:    "/root",
			requestPath: "/subdir/file.x",
			want:        "/tftpboot/subdir",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mount.HostPath(tt.rootPath, tt.requestPath)
			if got != tt.want {
				t.Errorf("HostPath(%q, %q) = %q, want %q",
					tt.rootPath, tt.requestPath, got, tt.want)
			}
		})
	}
}

func TestProxyDirector_AppendSuffix(t *testing.T) {
	tests := []struct {
		name     string
		mount    Mount
		reqPath  string
		wantPath string
	}{
		{
			name:     "suffix with matching slashes",
			mount:    Mount{Path: "/images", Proxy: "http://upstream/repo", PathIsPrefix: true, AppendSuffix: true},
			reqPath:  "/images/ubuntu/file.iso",
			wantPath: "/repo/ubuntu/file.iso",
		},
		{
			name:     "suffix with mismatched slashes",
			mount:    Mount{Path: "images", Proxy: "http://upstream/repo/", PathIsPrefix: true, AppendSuffix: true},
			reqPath:  "/images/ubuntu/file.iso",
			wantPath: "/repo/ubuntu/file.iso",
		},
		{
			name:     "no suffix",
			mount:    Mount{Path: "/images", Proxy: "http://upstream/repo", PathIsPrefix: true, AppendSuffix: false},
			reqPath:  "/images/ubuntu/file.iso",
			wantPath: "/repo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			director, err := tt.mount.ProxyDirector()
			if err != nil {
				t.Fatalf("ProxyDirector() error: %v", err)
			}
			req, _ := http.NewRequest("GET", "http://localhost"+tt.reqPath, nil)
			director(req)
			if req.URL.Path != tt.wantPath {
				t.Errorf("after director, URL.Path = %q, want %q", req.URL.Path, tt.wantPath)
			}
		})
	}
}

func TestProxyDirector_Escaping(t *testing.T) {
	tests := []struct {
		name        string
		mount       Mount
		reqURL      string
		wantEscaped string
	}{
		{
			name:        "target has escaped path, request has no escaping",
			mount:       Mount{Path: "/images", Proxy: "http://upstream/repo%2Fsub", PathIsPrefix: true, AppendSuffix: true},
			reqURL:      "http://localhost/images/ubuntu/file.iso",
			wantEscaped: "/repo%2Fsub/ubuntu/file.iso",
		},
		{
			// Matching and suffix extraction operate on the decoded request
			// path (same as GetMount), so a %2F in the request is treated as
			// a literal path separator rather than round-tripped verbatim.
			// This trades exact-byte preservation for a suffix that can
			// never desync Path from RawPath (see the RawPath-consistency
			// tests below).
			name:        "target has no escaping, request has escaped path",
			mount:       Mount{Path: "/images", Proxy: "http://upstream/repo", PathIsPrefix: true, AppendSuffix: true},
			reqURL:      "http://localhost/images/ubuntu%2Ffile.iso",
			wantEscaped: "/repo/ubuntu/file.iso",
		},
		{
			name:        "both target and request have escaped paths",
			mount:       Mount{Path: "/images", Proxy: "http://upstream/repo%2Fsub", PathIsPrefix: true, AppendSuffix: true},
			reqURL:      "http://localhost/images/ubuntu%2Ffile.iso",
			wantEscaped: "/repo%2Fsub/ubuntu/file.iso",
		},
		{
			name:        "neither has escaping",
			mount:       Mount{Path: "/images", Proxy: "http://upstream/repo", PathIsPrefix: true, AppendSuffix: true},
			reqURL:      "http://localhost/images/ubuntu/file.iso",
			wantEscaped: "/repo/ubuntu/file.iso",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			director, err := tt.mount.ProxyDirector()
			if err != nil {
				t.Fatalf("ProxyDirector() error: %v", err)
			}
			req, err := http.NewRequest("GET", tt.reqURL, nil)
			if err != nil {
				t.Fatalf("NewRequest() error: %v", err)
			}
			director(req)
			got := req.URL.EscapedPath()
			if got != tt.wantEscaped {
				t.Errorf("after director, URL.EscapedPath() = %q, want %q", got, tt.wantEscaped)
			}
		})
	}
}

func TestProxyDirector_TrailingSlashPreserved(t *testing.T) {
	// A directory-style request (trailing slash) must proxy to a
	// directory-style upstream URL, not have the slash silently dropped.
	mount := Mount{Path: "/", PathIsPrefix: true, AppendSuffix: true,
		Proxy: "http://upstream/dists/focal/netboot/amd64/"}
	director, err := mount.ProxyDirector()
	if err != nil {
		t.Fatalf("ProxyDirector() error: %v", err)
	}

	tests := []struct {
		reqPath  string
		wantPath string
	}{
		{"/", "/dists/focal/netboot/amd64/"},
		{"/subdir/", "/dists/focal/netboot/amd64/subdir/"},
		{"/subdir", "/dists/focal/netboot/amd64/subdir"},
	}
	for _, tt := range tests {
		t.Run(tt.reqPath, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://localhost"+tt.reqPath, nil)
			director(req)
			if req.URL.Path != tt.wantPath {
				t.Errorf("director(%q): Path = %q, want %q", tt.reqPath, req.URL.Path, tt.wantPath)
			}
		})
	}
}

func TestProxyDirector_DotDotCannotEscapeTarget(t *testing.T) {
	mount := Mount{Path: "/images", PathIsPrefix: true, AppendSuffix: true,
		Proxy: "http://upstream/repo/"}
	director, err := mount.ProxyDirector()
	if err != nil {
		t.Fatalf("ProxyDirector() error: %v", err)
	}
	req, _ := http.NewRequest("GET", "http://localhost/images/../../secret", nil)
	director(req)
	if !strings.HasPrefix(req.URL.Path, "/repo/") {
		t.Errorf("dot-dot request escaped proxy base: Path = %q", req.URL.Path)
	}
}

func TestProxyDirector_RawPathAlwaysConsistent(t *testing.T) {
	// Path and RawPath must always describe the same resource, or the
	// server silently sends whichever one it recomputes rather than what
	// was configured/requested.
	mount := Mount{Path: "/images", PathIsPrefix: true, AppendSuffix: true,
		Proxy: "http://upstream/repo%2Fsub"}
	director, err := mount.ProxyDirector()
	if err != nil {
		t.Fatalf("ProxyDirector() error: %v", err)
	}

	// Unlike TFTP filenames, a malformed percent-escape (e.g. "100%off")
	// never reaches the director for HTTP requests: net/http rejects it
	// while parsing the request line, before routing. That case is covered
	// for TFTP by TestTftpProxySuffix_InvalidEscapeIsSafe instead.
	reqPaths := []string{
		"/images/a b/file.iso",
		"/images/ubuntu%2Ffile.iso/x",
		"/images/../top",
	}
	for _, p := range reqPaths {
		t.Run(p, func(t *testing.T) {
			req, err := http.NewRequest("GET", "http://localhost"+p, nil)
			if err != nil {
				t.Fatalf("NewRequest() error: %v", err)
			}
			director(req)
			if req.URL.RawPath != "" {
				unescaped, err := url.PathUnescape(req.URL.RawPath)
				if err != nil {
					t.Fatalf("RawPath %q is not validly escaped: %v", req.URL.RawPath, err)
				}
				if unescaped != req.URL.Path {
					t.Errorf("RawPath %q does not decode to Path %q (decoded to %q)",
						req.URL.RawPath, req.URL.Path, unescaped)
				}
			}
		})
	}
}

func TestTftpProxySuffix_InvalidEscapeIsSafe(t *testing.T) {
	// A TFTP filename containing a literal '%' that isn't a valid escape
	// must not be silently dropped by url.JoinPath.
	mount := Mount{Path: "/images", PathIsPrefix: true, AppendSuffix: true}
	suffix, ok := mount.PathSuffix("images/100%off/file")
	if !ok {
		t.Fatalf("PathSuffix() ok = false, want true")
	}
	got, err := url.JoinPath("http://upstream/repo", EscapePathSuffix(suffix))
	if err != nil {
		t.Fatalf("JoinPath() error: %v", err)
	}
	want := "http://upstream/repo/100%25off/file"
	if got != want {
		t.Errorf("JoinPath() = %q, want %q", got, want)
	}
}

func TestGetMount_SlashVariations(t *testing.T) {
	// Ensure all slash variations of the same mount path and request path match correctly.
	pathVariations := []string{"/ubuntu", "ubuntu", "ubuntu/", "/ubuntu/"}
	for _, mountPath := range pathVariations {
		m := &Manifest{
			Mounts: []Mount{
				{Path: mountPath},
			},
		}
		for _, reqPath := range pathVariations {
			t.Run(mountPath+"_vs_"+reqPath, func(t *testing.T) {
				_, err := m.GetMount(reqPath)
				if err != nil {
					t.Errorf("GetMount(%q) with mount.Path=%q should match, got error: %v",
						reqPath, mountPath, err)
				}
			})
		}
	}
}
