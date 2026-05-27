package db

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"

	"github.com/neko233/kanban233/internal/config"
)

func (s *Store) SeedDefaults(ctx context.Context, cfg config.AuthConfig) error {
	if cfg.DefaultUser.Username == "" {
		return nil
	}
	user, err := s.GetUserByUsername(ctx, cfg.DefaultUser.Username)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if errors.Is(err, ErrNotFound) {
		hash, err := bcrypt.GenerateFromPassword([]byte(cfg.DefaultUser.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		user, err = s.CreateUser(ctx, cfg.DefaultUser.Username, string(hash))
		if err != nil {
			return err
		}
	}
	if err := s.SeedDemoContent(ctx, user.ID); err != nil {
		return err
	}
	return s.seedPostInstall(ctx, user.ID)
}

func (s *Store) seedPostInstall(ctx context.Context, ownerID int64) error {
	if err := s.EnsureAllGroupsHaveKanban(ctx, ownerID); err != nil {
		return err
	}
	if err := s.seedMyProjectWelcome(ctx, ownerID); err != nil {
		return err
	}
	return s.seedDemoCardWorkers(ctx, ownerID)
}
