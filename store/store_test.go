package store

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/DSpeichert/netbootd/manifest"
)

// storeFactory builds a fresh Store of one backend for the shared test suite.
type storeFactory struct {
	name string
	make func(t *testing.T) Store
}

func allBackends(t *testing.T) []storeFactory {
	return []storeFactory{
		{"memory", func(t *testing.T) Store {
			s, err := NewMemoryStore(Config{})
			if err != nil {
				t.Fatalf("NewMemoryStore: %v", err)
			}
			return s
		}},
		{"disk", func(t *testing.T) Store {
			s, err := NewDiskStore(Config{DiskPath: t.TempDir()})
			if err != nil {
				t.Fatalf("NewDiskStore: %v", err)
			}
			return s
		}},
		{"sqlite", func(t *testing.T) Store {
			s, err := NewSQLiteStore(Config{SQLitePath: filepath.Join(t.TempDir(), "test.db")})
			if err != nil {
				t.Fatalf("NewSQLiteStore: %v", err)
			}
			return s
		}},
	}
}

func testManifest(t *testing.T, id, ip, mac string) manifest.Manifest {
	t.Helper()
	hw, err := manifest.ParseMAC(mac)
	if err != nil {
		t.Fatalf("ParseMAC(%q): %v", mac, err)
	}
	return manifest.Manifest{
		ID:   id,
		IPv4: ipWithMask(ip, 24),
		MAC:  []manifest.HardwareAddr{hw},
	}
}

// TestStore_Contract exercises the behavior every backend must share.
func TestStore_Contract(t *testing.T) {
	for _, b := range allBackends(t) {
		t.Run(b.name, func(t *testing.T) {
			s := b.make(t)
			defer s.Close()

			m := testManifest(t, "host1", "192.168.1.10", "aa:bb:cc:dd:ee:ff")
			if err := s.PutManifest(m); err != nil {
				t.Fatalf("PutManifest: %v", err)
			}

			if g := s.Find("host1"); g == nil || g.ID != "host1" {
				t.Fatalf("Find: got %+v", g)
			}
			if g := s.FindByIP(net.ParseIP("192.168.1.10")); g == nil || g.ID != "host1" {
				t.Fatalf("FindByIP: got %+v", g)
			}
			mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
			if g := s.FindByMAC(mac); g == nil || g.ID != "host1" {
				t.Fatalf("FindByMAC: got %+v", g)
			}
			if all := s.GetAll(); len(all) != 1 || all["host1"] == nil {
				t.Fatalf("GetAll: got %+v", all)
			}

			// SetSuspended persists the flag.
			if err := s.SetSuspended("host1", true); err != nil {
				t.Fatalf("SetSuspended: %v", err)
			}
			if g := s.Find("host1"); g == nil || !g.Suspended {
				t.Fatalf("SetSuspended not reflected: %+v", g)
			}

			// Re-Put with a changed IP must not leave a stale FindByIP entry.
			m2 := testManifest(t, "host1", "192.168.1.20", "aa:bb:cc:dd:ee:ff")
			if err := s.PutManifest(m2); err != nil {
				t.Fatalf("PutManifest re-put: %v", err)
			}
			if g := s.FindByIP(net.ParseIP("192.168.1.10")); g != nil {
				t.Fatalf("stale FindByIP after IP change: %+v", g)
			}
			if g := s.FindByIP(net.ParseIP("192.168.1.20")); g == nil {
				t.Fatalf("FindByIP after IP change: not found")
			}

			// Forget removes everything.
			if err := s.ForgetManifest("host1"); err != nil {
				t.Fatalf("ForgetManifest: %v", err)
			}
			if g := s.Find("host1"); g != nil {
				t.Fatalf("Find after forget: %+v", g)
			}
			if g := s.FindByMAC(mac); g != nil {
				t.Fatalf("FindByMAC after forget: %+v", g)
			}
			// Forget of a missing ID is a no-op.
			if err := s.ForgetManifest("nope"); err != nil {
				t.Fatalf("ForgetManifest(missing): %v", err)
			}
		})
	}
}

// TestDiskStore_PersistsAcrossRestart verifies files survive a new store on the
// same directory, and that Forget removes the file.
func TestDiskStore_PersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()

	s1, err := NewDiskStore(Config{DiskPath: dir})
	if err != nil {
		t.Fatalf("NewDiskStore: %v", err)
	}
	if err := s1.PutManifest(testManifest(t, "keep", "10.0.0.5", "aa:bb:cc:dd:ee:01")); err != nil {
		t.Fatalf("PutManifest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.yml")); err != nil {
		t.Fatalf("expected keep.yml on disk: %v", err)
	}
	_ = s1.Close()

	// Fresh store on the same directory restores the manifest.
	s2, err := NewDiskStore(Config{DiskPath: dir})
	if err != nil {
		t.Fatalf("NewDiskStore restore: %v", err)
	}
	defer s2.Close()
	if g := s2.Find("keep"); g == nil || g.ID != "keep" {
		t.Fatalf("manifest not restored: %+v", g)
	}

	if err := s2.ForgetManifest("keep"); err != nil {
		t.Fatalf("ForgetManifest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.yml")); !os.IsNotExist(err) {
		t.Fatalf("keep.yml should be removed, stat err=%v", err)
	}
}

// TestDiskStore_RejectsUnsafeID ensures a traversal-y ID cannot escape the dir.
func TestDiskStore_RejectsUnsafeID(t *testing.T) {
	dir := t.TempDir()
	s, err := NewDiskStore(Config{DiskPath: dir})
	if err != nil {
		t.Fatalf("NewDiskStore: %v", err)
	}
	defer s.Close()

	m := testManifest(t, "../escape", "10.0.0.9", "aa:bb:cc:dd:ee:02")
	if err := s.PutManifest(m); err == nil {
		t.Fatalf("PutManifest with traversal ID should fail")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.yml")); err == nil {
		t.Fatalf("traversal wrote a file outside the store dir")
	}
}

func TestNewStore_UnknownBackend(t *testing.T) {
	if _, err := NewStore(Config{Backend: "redis"}); err == nil {
		t.Fatalf("expected error for unknown backend")
	}
}
