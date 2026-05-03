// Package store provides Meta service database operations using GORM.
package store

import (
	"errors"

	"gorm.io/gorm"
)

// Common store errors.
var (
	ErrNotFound = errors.New("record not found")
)

// Store wraps separate write and read GORM handles.
//
// SQLite uses different connection pools for writes and reads. Postgres and
// in-memory test databases can pass nil for readDB, which reuses writeDB.
type Store struct {
	writeDB *gorm.DB
	readDB  *gorm.DB
}

// New creates a Store with separate write and read handles.
func New(writeDB, readDB *gorm.DB) *Store {
	if readDB == nil {
		readDB = writeDB
	}
	return &Store{writeDB: writeDB, readDB: readDB}
}

// DB returns the write database handle for advanced write-side operations.
func (s *Store) DB() *gorm.DB {
	return s.writeDB
}

// ReadDB returns the read database handle for advanced read-side operations.
func (s *Store) ReadDB() *gorm.DB {
	return s.readDB
}

func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
