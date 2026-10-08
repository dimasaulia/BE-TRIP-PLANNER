// Package storage keeps uploaded images on local disk under
// UPLOAD_DIR/<trip_id>/<stored_name>.
package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/platform/config"
)

var storedNamePattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(jpg|png|webp|gif)$`)

// ValidStoredName guards every path built from user input.
func ValidStoredName(name string) bool {
	return storedNamePattern.MatchString(name)
}

type Store struct {
	dir string
}

func New(cfg config.Config) *Store {
	return &Store{dir: cfg.Upload.Dir}
}

func (s *Store) TripDir(tripID uuid.UUID) string {
	return filepath.Join(s.dir, tripID.String())
}

func (s *Store) EnsureTripDir(tripID uuid.UUID) (string, error) {
	dir := s.TripDir(tripID)
	return dir, os.MkdirAll(dir, 0o755)
}

func (s *Store) Path(tripID uuid.UUID, storedName string) (string, error) {
	if !ValidStoredName(storedName) {
		return "", errors.New("invalid stored name")
	}
	return filepath.Join(s.TripDir(tripID), storedName), nil
}

func (s *Store) Open(tripID uuid.UUID, storedName string) (*os.File, error) {
	path, err := s.Path(tripID, storedName)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *Store) Remove(tripID uuid.UUID, storedName string) error {
	path, err := s.Path(tripID, storedName)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// RemoveTrip deletes the whole directory of a trip.
func (s *Store) RemoveTrip(tripID uuid.UUID) error {
	return os.RemoveAll(s.TripDir(tripID))
}

// Copy duplicates a stored file into another trip under a new name.
func (s *Store) Copy(srcTrip uuid.UUID, srcName string, dstTrip uuid.UUID, dstName string) error {
	source, err := s.Open(srcTrip, srcName)
	if err != nil {
		return err
	}
	defer source.Close()

	dir, err := s.EnsureTripDir(dstTrip)
	if err != nil {
		return err
	}
	if !ValidStoredName(dstName) {
		return errors.New("invalid stored name")
	}

	temp, err := os.CreateTemp(dir, ".copy-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()

	if _, err := io.Copy(temp, source); err != nil {
		temp.Close()
		os.Remove(tempName)
		return err
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempName)
		return err
	}

	if err := os.Rename(tempName, filepath.Join(dir, dstName)); err != nil {
		os.Remove(tempName)
		return err
	}
	return nil
}
