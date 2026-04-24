package memory

import (
	"context"
	"fmt"
	"sync"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// RealUserRepo is a map-backed UserRepository for unit tests.
type RealUserRepo struct {
	mu      sync.RWMutex
	byID    map[string]repository.User
	byEmail map[string]string // email -> id
}

func NewRealUserRepo() *RealUserRepo {
	return &RealUserRepo{
		byID:    make(map[string]repository.User),
		byEmail: make(map[string]string),
	}
}

func (r *RealUserRepo) Seed(users ...repository.User) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range users {
		r.byID[u.ID] = u
		r.byEmail[u.Email] = u.ID
	}
}

func (r *RealUserRepo) GetByID(_ context.Context, id string) (repository.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byID[id]
	if !ok {
		return repository.User{}, fmt.Errorf("user %q: %w", id, apperr.ErrNotFound)
	}
	return u, nil
}

func (r *RealUserRepo) GetByEmail(_ context.Context, email string) (repository.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byEmail[email]
	if !ok {
		return repository.User{}, fmt.Errorf("user %q: %w", email, apperr.ErrNotFound)
	}
	return r.byID[id], nil
}

func (r *RealUserRepo) Create(_ context.Context, u repository.User) (repository.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byEmail[u.Email]; exists {
		return repository.User{}, fmt.Errorf("email already in use: %w", apperr.ErrConflict)
	}
	r.byID[u.ID] = u
	r.byEmail[u.Email] = u.ID
	return u, nil
}

func (r *RealUserRepo) Update(_ context.Context, u repository.User) (repository.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[u.ID]; !ok {
		return repository.User{}, fmt.Errorf("user %q: %w", u.ID, apperr.ErrNotFound)
	}
	r.byID[u.ID] = u
	return u, nil
}

func (r *RealUserRepo) RunInTx(_ context.Context, fn func(repository.UserRepository) error) error {
	return fn(r)
}
