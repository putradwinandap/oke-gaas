package access

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
)

var (
	ErrInvalidProjectID = errors.New("project api key project id is required")
	ErrNotFound         = errors.New("project api key not found")
	ErrUnauthorized     = errors.New("invalid project api key")
)

// ProjectKey stores only a one-way hash of a Project API key.
type ProjectKey struct {
	projectID  string
	secretHash [sha256.Size]byte
}

// Generate creates a new secret suitable for returning once to an API client.
func Generate(projectID string) (*ProjectKey, string, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, "", ErrInvalidProjectID
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", err
	}
	secret := "gaas_" + base64.RawURLEncoding.EncodeToString(raw)
	return &ProjectKey{projectID: projectID, secretHash: sha256.Sum256([]byte(secret))}, secret, nil
}

// Restore reconstructs a persisted Project API key hash.
func Restore(projectID string, hash []byte) (*ProjectKey, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, ErrInvalidProjectID
	}
	if len(hash) != sha256.Size {
		return nil, errors.New("project api key hash must be sha256")
	}
	var value [sha256.Size]byte
	copy(value[:], hash)
	return &ProjectKey{projectID: projectID, secretHash: value}, nil
}

func (k ProjectKey) ProjectID() string { return k.projectID }

func (k ProjectKey) Hash() []byte {
	return append([]byte(nil), k.secretHash[:]...)
}

// Verify compares a caller-provided secret without exposing the persisted hash.
func (k ProjectKey) Verify(secret string) bool {
	if strings.TrimSpace(secret) == "" {
		return false
	}
	candidate := sha256.Sum256([]byte(secret))
	return subtle.ConstantTimeCompare(k.secretHash[:], candidate[:]) == 1
}

type Repository interface {
	Save(ctx context.Context, key *ProjectKey) error
	GetByProjectID(ctx context.Context, projectID string) (*ProjectKey, error)
}

// Service authenticates requests at the Project tenant boundary.
type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Authenticate(ctx context.Context, projectID, secret string) error {
	if s == nil || s.repository == nil {
		return errors.New("project api key repository is required")
	}
	if strings.TrimSpace(secret) == "" {
		return ErrUnauthorized
	}
	key, err := s.repository.GetByProjectID(ctx, projectID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrUnauthorized
		}
		return err
	}
	if !key.Verify(secret) {
		return ErrUnauthorized
	}
	return nil
}
