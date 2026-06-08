package manifest

import (
	"net/http"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"slash only", "/", ""},
		{"trailing slash", "foo/", "foo"},
		{"leading slash", "/foo", "foo"},
		{"both slashes", "/foo/", "foo"},
		{"multiple leading", "///foo", "foo"},
		{"nested", "/foo/bar/baz", "foo/bar/baz"},
		{"nested trailing", "/foo/bar/", "foo/bar"},
		{"no slashes", "foo/bar", "foo/bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePath(tt.in)
			if got != tt.want {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.in, got, tt.want)
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
	}{
		{"exact match", "/foo", "/foo", ""},
		{"exact match no slashes", "foo", "foo", ""},
		{"suffix extraction", "/foo", "/foo/bar", "/bar"},
		{"mismatched leading slashes", "foo", "/foo/bar", "/bar"},
		{"mount with slash request without", "/foo", "foo/bar/baz", "/bar/baz"},
		{"trailing slash on mount", "foo/", "/foo/bar", "/bar"},
		{"empty mount", "", "/foo/bar", "/foo/bar"},
		{"empty mount empty request", "", "", ""},
		{"deep suffix", "/a/b", "/a/b/c/d/e", "/c/d/e"},
		{"partial segment no match", "foo", "foobar", "/foobar"},
		{"partial segment nested no match", "foo/bar", "foo/barbaz", "/foo/barbaz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Mount{Path: tt.mountPath}
			got := m.PathSuffix(tt.reqPath)
			if got != tt.want {
				t.Errorf("Mount{Path: %q}.PathSuffix(%q) = %q, want %q",
					tt.mountPath, tt.reqPath, got, tt.want)
			}
		})
	}
}

func TestHostPath(t *testing.T) {
	tests := []struct {
		name         string
		mount        Mount
		rootPath     string
		requestPath  string
		wantContains string
	}{
		{
			name:         "append suffix with slash mismatch",
			mount:        Mount{Path: "/subdir", AppendSuffix: true, LocalDir: "/tftpboot"},
			rootPath:     "/root",
			requestPath:  "/subdir/file.x",
			wantContains: "/tftpboot/file.x",
		},
		{
			name:         "append suffix without leading slash",
			mount:        Mount{Path: "subdir", AppendSuffix: true, LocalDir: "/tftpboot"},
			rootPath:     "/root",
			requestPath:  "/subdir/file.x",
			wantContains: "/tftpboot/file.x",
		},
		{
			name:         "no append suffix",
			mount:        Mount{Path: "/subdir", AppendSuffix: false, LocalDir: "/tftpboot"},
			rootPath:     "/root",
			requestPath:  "/subdir/file.x",
			wantContains: "/tftpboot/subdir",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mount.HostPath(tt.rootPath, tt.requestPath)
			if got != tt.wantContains {
				t.Errorf("HostPath(%q, %q) = %q, want %q",
					tt.rootPath, tt.requestPath, got, tt.wantContains)
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
