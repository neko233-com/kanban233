package db

import (
	"context"

	"golang.org/x/crypto/bcrypt"

	"github.com/neko233/kanban233/internal/config"
)

func (s *Store) SeedDefaults(ctx context.Context, cfg config.AuthConfig) error {
	if cfg.DefaultUser.Username == "" {
		return nil
	}
	_, err := s.GetUserByUsername(ctx, cfg.DefaultUser.Username)
	if err == nil {
		return nil
	}
	if err != ErrNotFound {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.DefaultUser.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.CreateUser(ctx, cfg.DefaultUser.Username, string(hash))
	return err
}
