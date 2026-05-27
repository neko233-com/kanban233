package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/neko233/kanban233/internal/config"
	"github.com/neko233/kanban233/internal/db"
	"github.com/neko233/kanban233/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	store   *db.Store
	secret  []byte
	ttl     time.Duration
	regOpen bool
}

type contextKey string

const UserIDKey contextKey = "userID"

func NewService(store *db.Store, cfg config.AuthConfig) *Service {
	return &Service{
		store:   store,
		secret:  []byte(cfg.JWTSecret),
		ttl:     time.Duration(cfg.TokenTTLHours) * time.Hour,
		regOpen: cfg.RegistrationOpen,
	}
}

func (s *Service) RegistrationOpen() bool {
	return s.regOpen
}

func (s *Service) Register(ctx context.Context, req models.RegisterRequest) (*models.AuthResponse, error) {
	if !s.regOpen {
		return nil, ErrRegistrationClosed
	}
	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) < 3 || len(req.Password) < 6 {
		return nil, ErrInvalidCredentials
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user, err := s.store.CreateUser(ctx, req.Username, string(hash))
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			return nil, ErrUsernameTaken
		}
		return nil, err
	}

	token, err := s.issueToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{Token: token, User: *user}, nil
}

func (s *Service) Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
	user, err := s.store.GetUserByUsername(ctx, strings.TrimSpace(req.Username))
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.issueToken(user.ID)
	if err != nil {
		return nil, err
	}

	user.PasswordHash = ""
	return &models.AuthResponse{Token: token, User: *user}, nil
}

func (s *Service) Me(ctx context.Context, userID int64) (*models.User, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	user.PasswordHash = ""
	return user, nil
}

func (s *Service) issueToken(userID int64) (string, error) {
	claims := jwt.MapClaims{
		"sub": fmt.Sprintf("%d", userID),
		"exp": time.Now().Add(s.ttl).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *Service) ParseToken(tokenStr string) (int64, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil || !token.Valid {
		return 0, ErrUnauthorized
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, ErrUnauthorized
	}
	sub, ok := claims["sub"].(string)
	if !ok {
		return 0, ErrUnauthorized
	}
	var userID int64
	if _, err := fmt.Sscanf(sub, "%d", &userID); err != nil {
		return 0, ErrUnauthorized
	}
	return userID, nil
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := s.userIDFromRequest(r)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) userIDFromRequest(r *http.Request) (int64, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return 0, ErrUnauthorized
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return 0, ErrUnauthorized
	}
	return s.ParseToken(parts[1])
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(UserIDKey).(int64)
	return id, ok
}

var (
	ErrRegistrationClosed = errors.New("registration closed")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUsernameTaken      = errors.New("username taken")
	ErrUnauthorized       = errors.New("unauthorized")
)
