package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DSpeichert/netbootd/store"
)

const testManifestYAML = `id: host1
ipv4: 192.168.1.10/24
mac:
  - aa:bb:cc:dd:ee:ff
`

func do(t *testing.T, srv *Server, method, target, body string, remote string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	if remote != "" {
		r.RemoteAddr = remote
	}
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, r)
	return w
}

// TestAPI_PersistenceRoundTrip drives the HTTP API against a sqlite store and
// verifies that a created manifest and a suspend toggle survive a store reopen.
func TestAPI_PersistenceRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "api.sqlite3")

	st, err := store.NewStore(store.Config{Backend: "sqlite", SQLitePath: dbPath})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	srv, err := NewServer(st, "", "")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Create a manifest.
	if w := do(t, srv, "PUT", "/api/manifests/host1", testManifestYAML, ""); w.Code != http.StatusCreated {
		t.Fatalf("PUT manifest: got %d, body %q", w.Code, w.Body.String())
	}
	if m := st.Find("host1"); m == nil || m.ID != "host1" {
		t.Fatalf("manifest not stored: %+v", m)
	}

	// Suspend boot for that host (IP taken from RemoteAddr).
	if w := do(t, srv, "POST", "/api/self/suspend-boot", "", "192.168.1.10:5000"); w.Code != http.StatusOK {
		t.Fatalf("suspend-boot: got %d, body %q", w.Code, w.Body.String())
	}
	if m := st.Find("host1"); m == nil || !m.Suspended {
		t.Fatalf("suspend not persisted in store: %+v", m)
	}

	// Reopen the sqlite store on the same file: manifest + suspend flag persist.
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	st2, err := store.NewStore(store.Config{Backend: "sqlite", SQLitePath: dbPath})
	if err != nil {
		t.Fatalf("reopen NewStore: %v", err)
	}
	defer st2.Close()
	if m := st2.Find("host1"); m == nil || !m.Suspended {
		t.Fatalf("manifest/suspend did not survive reopen: %+v", m)
	}

	// Delete via a second server bound to the reopened store.
	srv2, err := NewServer(st2, "", "")
	if err != nil {
		t.Fatalf("NewServer2: %v", err)
	}
	if w := do(t, srv2, "DELETE", "/api/manifests/host1", "", ""); w.Code != http.StatusNoContent {
		t.Fatalf("DELETE manifest: got %d, body %q", w.Code, w.Body.String())
	}
	if m := st2.Find("host1"); m != nil {
		t.Fatalf("manifest still present after delete: %+v", m)
	}
}
