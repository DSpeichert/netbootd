package store

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"path/filepath"

	"github.com/DSpeichert/netbootd/manifest"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

// sqliteStore persists manifests in a SQLite database and answers lookups with
// SQL queries (no in-memory index). Manifests are stored as YAML blobs; IP and
// MAC lookups are backed by dedicated columns / a join table.
type sqliteStore struct {
	db     *sql.DB
	logger zerolog.Logger
}

// NewSQLiteStore returns a SQLite-backed Store at cfg.SQLitePath.
func NewSQLiteStore(cfg Config) (Store, error) {
	if cfg.SQLitePath == "" {
		return nil, errors.New("SQLitePath is required for sqlite backend")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.SQLitePath), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", cfg.SQLitePath)
	if err != nil {
		return nil, err
	}
	// SQLite tolerates concurrency poorly through database/sql's pool; a single
	// connection avoids spurious "database is locked" errors.
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA foreign_keys = ON;",
	} {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	s := &sqliteStore{
		db:     db,
		logger: log.With().Str("module", "store").Str("backend", "sqlite").Logger(),
	}
	if err := s.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *sqliteStore) initSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS manifests (
			id      TEXT PRIMARY KEY,
			data    BLOB NOT NULL,
			ip_norm TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_manifests_ip ON manifests(ip_norm);`,
		`CREATE TABLE IF NOT EXISTS manifest_macs (
			mac TEXT NOT NULL,
			id  TEXT NOT NULL REFERENCES manifests(id) ON DELETE CASCADE,
			PRIMARY KEY (mac, id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_manifest_macs_mac ON manifest_macs(mac);`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// sqliteIPNorm is the hex-encoded 16-byte form used as the ip_norm column value.
func sqliteIPNorm(ip net.IP) string {
	if ip == nil {
		return ""
	}
	v16 := ip.To16()
	if v16 == nil {
		return ""
	}
	return hex.EncodeToString(v16)
}

func (s *sqliteStore) LoadFromDirectory(path, rootPath string) error {
	return loadFromDirectory(s.PutManifest, path, rootPath, s.logger)
}

func (s *sqliteStore) PutManifest(m manifest.Manifest) error {
	if err := validateManifest(m); err != nil {
		return err
	}
	blob, err := encodeManifest(m)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`INSERT INTO manifests(id, data, ip_norm) VALUES(?,?,?)
		 ON CONFLICT(id) DO UPDATE SET data=excluded.data, ip_norm=excluded.ip_norm;`,
		m.ID, blob, sqliteIPNorm(m.IPv4.IP),
	); err != nil {
		return err
	}
	// Rebuild this manifest's MAC rows.
	if _, err := tx.Exec(`DELETE FROM manifest_macs WHERE id = ?;`, m.ID); err != nil {
		return err
	}
	for _, mac := range m.MAC {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO manifest_macs(mac, id) VALUES(?,?);`, mac.String(), m.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *sqliteStore) ForgetManifest(id string) error {
	// manifest_macs rows cascade via the foreign key.
	_, err := s.db.Exec(`DELETE FROM manifests WHERE id = ?;`, id)
	return err
}

func (s *sqliteStore) SetSuspended(id string, suspended bool) error {
	return setSuspended(s, id, suspended)
}

// queryManifest runs a single-row query returning the data blob and decodes it.
// A missing row returns nil; any other error is logged and also returns nil, so
// a transient DB problem cannot be mistaken for a valid manifest.
func (s *sqliteStore) queryManifest(what, query string, args ...any) *manifest.Manifest {
	var blob []byte
	switch err := s.db.QueryRow(query, args...).Scan(&blob); {
	case err == sql.ErrNoRows:
		return nil
	case err != nil:
		s.logger.Error().Err(err).Str("lookup", what).Msg("manifest query failed")
		return nil
	}
	m, err := decodeManifest(blob)
	if err != nil {
		s.logger.Error().Err(err).Str("lookup", what).Msg("cannot decode stored manifest")
		return nil
	}
	return &m
}

func (s *sqliteStore) Find(id string) *manifest.Manifest {
	return s.queryManifest("id", `SELECT data FROM manifests WHERE id = ?;`, id)
}

func (s *sqliteStore) FindByIP(ip net.IP) *manifest.Manifest {
	return s.queryManifest("ip", `SELECT data FROM manifests WHERE ip_norm = ? LIMIT 1;`, sqliteIPNorm(ip))
}

func (s *sqliteStore) FindByMAC(mac net.HardwareAddr) *manifest.Manifest {
	return s.queryManifest("mac",
		`SELECT m.data FROM manifests m
		 JOIN manifest_macs mm ON mm.id = m.id
		 WHERE mm.mac = ? LIMIT 1;`, mac.String())
}

func (s *sqliteStore) GetAll() map[string]*manifest.Manifest {
	res := make(map[string]*manifest.Manifest)
	rows, err := s.db.Query(`SELECT id, data FROM manifests;`)
	if err != nil {
		s.logger.Error().Err(err).Msg("GetAll query failed")
		return res
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			s.logger.Error().Err(err).Msg("GetAll row scan failed")
			continue
		}
		m, err := decodeManifest(blob)
		if err != nil {
			s.logger.Error().Err(err).Str("id", id).Msg("cannot decode stored manifest")
			continue
		}
		mc := m
		res[id] = &mc
	}
	if err := rows.Err(); err != nil {
		s.logger.Error().Err(err).Msg("GetAll row iteration failed")
	}
	return res
}

func (s *sqliteStore) Close() error { return s.db.Close() }
