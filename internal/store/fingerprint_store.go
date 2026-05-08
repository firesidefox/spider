package store

import (
	"database/sql"
	"time"

	"github.com/spiderai/spider/internal/models"
)

type FingerprintStore struct {
	db *sql.DB
}

func NewFingerprintStore(db *sql.DB) *FingerprintStore {
	return &FingerprintStore{db: db}
}

func (s *FingerprintStore) Upsert(fp *models.Fingerprint) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO host_fingerprints
		(host_id,ssh_host_key,system_version,hardware_id,api_signature,status,snapshot_at)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(host_id) DO UPDATE SET
			ssh_host_key=excluded.ssh_host_key,
			system_version=excluded.system_version,
			hardware_id=excluded.hardware_id,
			api_signature=excluded.api_signature,
			status=excluded.status,
			snapshot_at=excluded.snapshot_at`,
		fp.HostID, fp.SSHHostKey, fp.SystemVersion, fp.HardwareID,
		fp.APISignature, fp.Status, now)
	return err
}

func (s *FingerprintStore) Get(hostID string) (*models.Fingerprint, error) {
	var fp models.Fingerprint
	err := s.db.QueryRow(`SELECT host_id,ssh_host_key,system_version,hardware_id,api_signature,status,snapshot_at
		FROM host_fingerprints WHERE host_id=?`, hostID).
		Scan(&fp.HostID, &fp.SSHHostKey, &fp.SystemVersion, &fp.HardwareID,
			&fp.APISignature, &fp.Status, &fp.SnapshotAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &fp, err
}

func (s *FingerprintStore) MarkChanged(hostID string) error {
	_, err := s.db.Exec(`UPDATE host_fingerprints SET status='changed' WHERE host_id=?`, hostID)
	return err
}
