package store

import (
	"net"
	"path/filepath"
	"testing"

	"github.com/DSpeichert/netbootd/manifest"
)

func ipWithMask(ip string, maskBits int) manifest.IPWithNet {
	mask := net.CIDRMask(maskBits, 32)
	return manifest.IPWithNet{
		IP: net.ParseIP(ip),
		Net: net.IPNet{
			IP:   net.IPv4zero,
			Mask: mask,
		},
	}
}

func mustParseMAC(t *testing.T, s string) (manifest.HardwareAddr, net.HardwareAddr) {
	t.Helper()
	m1, err := manifest.ParseMAC(s)
	if err != nil {
		t.Fatalf("manifest.ParseMAC: %v", err)
	}
	m2, err := net.ParseMAC(s)
	if err != nil {
		t.Fatalf("net.ParseMAC: %v", err)
	}
	return m1, m2
}

func newSQLiteTestStore(t *testing.T) Store {
	t.Helper()
	td := t.TempDir()
	p := filepath.Join(td, "test.db")
	st, err := NewSQLiteStore(Config{SQLitePath: p})
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	return st
}

func TestSQLiteStore_PutAndFindByMAC(t *testing.T) {
	st := newSQLiteTestStore(t)
	m1HW, m1Std := mustParseMAC(t, "aa:bb:cc:dd:ee:ff")

	m := manifest.Manifest{
		ID:   "host1",
		IPv4: ipWithMask("192.168.50.10", 24),
		MAC:  []manifest.HardwareAddr{m1HW},
	}
	if err := st.PutManifest(m); err != nil {
		t.Fatalf("PutManifest: %v", err)
	}

	got := st.FindByMAC(m1Std)
	if got == nil || got.ID != m.ID {
		t.Fatalf("FindByMAC got %+v, want ID %q", got, m.ID)
	}

	// sanity: other finders
	if g := st.Find("host1"); g == nil || g.ID != "host1" {
		t.Fatalf("Find(ID) failed, got %+v", g)
	}
	if g := st.FindByIP(net.ParseIP("192.168.50.10")); g == nil || g.ID != "host1" {
		t.Fatalf("FindByIP failed, got %+v", g)
	}
	if all := st.GetAll(); all["host1"].ID != "host1" {
		t.Fatalf("GetAll missing host1: %+v", all)
	}
}

func TestSQLiteStore_MultipleMACsAndNoSubstringCollision(t *testing.T) {
	st := newSQLiteTestStore(t)
	// First manifest with MAC that is suffix of the second's MAC when naively searched
	hwA1, stdA1 := mustParseMAC(t, "aa:bb:cc:dd:ee:ff")
	hwB1, stdB1 := mustParseMAC(t, "11:aa:bb:cc:dd:ee")

	mA := manifest.Manifest{ID: "A", IPv4: ipWithMask("10.0.0.2", 24), MAC: []manifest.HardwareAddr{hwA1}}
	mB := manifest.Manifest{ID: "B", IPv4: ipWithMask("10.0.0.3", 24), MAC: []manifest.HardwareAddr{hwB1}}
	if err := st.PutManifest(mA); err != nil {
		t.Fatalf("PutManifest A: %v", err)
	}
	if err := st.PutManifest(mB); err != nil {
		t.Fatalf("PutManifest B: %v", err)
	}

	if got := st.FindByMAC(stdA1); got == nil || got.ID != "A" {
		t.Fatalf("FindByMAC(stdA1) got %+v, want A", got)
	}
	if got := st.FindByMAC(stdB1); got == nil || got.ID != "B" {
		t.Fatalf("FindByMAC(stdB1) got %+v, want B", got)
	}
}

func TestSQLiteStore_ForgetManifestRemoves(t *testing.T) {
	st := newSQLiteTestStore(t)
	hw, std := mustParseMAC(t, "de:ad:be:ef:00:01")
	m := manifest.Manifest{ID: "gone", IPv4: ipWithMask("172.16.0.5", 16), MAC: []manifest.HardwareAddr{hw}}
	if err := st.PutManifest(m); err != nil {
		t.Fatalf("PutManifest: %v", err)
	}

	if st.Find("gone") == nil {
		t.Fatalf("Find before delete failed")
	}
	if err := st.ForgetManifest("gone"); err != nil {
		t.Fatalf("ForgetManifest: %v", err)
	}
	if st.Find("gone") != nil {
		t.Fatalf("Find after delete should be nil")
	}
	if st.FindByMAC(std) != nil {
		t.Fatalf("FindByMAC after delete should be nil")
	}
}
