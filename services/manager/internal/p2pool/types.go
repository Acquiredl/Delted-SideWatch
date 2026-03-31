package p2pool

import "time"

// IndexedShare represents a share stored in the database.
type IndexedShare struct {
	ID              int64
	Sidechain       string
	MinerAddress    string
	WorkerName      string
	SidechainHeight uint64
	Difficulty      uint64
	IsUncle         bool
	SoftwareID      *int16
	SoftwareVersion *string
	CreatedAt       time.Time
}

// IndexedBlock represents a P2Pool-found block stored in the database.
type IndexedBlock struct {
	ID                 int64
	MainHeight         uint64
	MainHash           string
	SidechainHeight    uint64
	CoinbaseReward     uint64
	Effort             float64
	CoinbasePrivateKey *string
	FoundAt            time.Time
}
