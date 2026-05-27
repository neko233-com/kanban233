package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Auth     AuthConfig     `yaml:"auth"`
	Database DatabaseConfig `yaml:"database"`
	Project  ProjectConfig  `yaml:"project"`
	Agent    AgentConfig    `yaml:"agent"`
}

type AgentConfig struct {
	Enabled  bool   `yaml:"enabled"`
	APIToken string `yaml:"api_token"`
}

type DefaultUserConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type ProjectConfig struct {
	DefaultJoinMode string `yaml:"default_join_mode"`
}

type ServerConfig struct {
	Addr      string `yaml:"addr"`
	StaticDir string `yaml:"static_dir"`
}

type AuthConfig struct {
	RegistrationOpen bool              `yaml:"registration_open"`
	JWTSecret        string            `yaml:"jwt_secret"`
	TokenTTLHours    int               `yaml:"token_ttl_hours"`
	DefaultUser      DefaultUserConfig `yaml:"default_user"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := defaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Addr:      ":61333",
			StaticDir: "./web",
		},
		Auth: AuthConfig{
			RegistrationOpen: true,
			JWTSecret:        "change-me-in-production",
			TokenTTLHours:    168,
			DefaultUser: DefaultUserConfig{
				Username: "root",
				Password: "root",
			},
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    "./data/kanban.db",
		},
		Project: ProjectConfig{
			DefaultJoinMode: "free",
		},
		Agent: AgentConfig{
			Enabled:  true,
			APIToken: "kanban-agent-intranet",
		},
	}
}

func (c *Config) validate() error {
	if c.Server.Addr == "" {
		return fmt.Errorf("server.addr is required")
	}
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("auth.jwt_secret is required")
	}
	if c.Auth.TokenTTLHours <= 0 {
		c.Auth.TokenTTLHours = 168
	}
	switch c.Database.Driver {
	case "sqlite", "postgres":
	default:
		return fmt.Errorf("unsupported database driver: %s", c.Database.Driver)
	}
	if c.Database.DSN == "" {
		return fmt.Errorf("database.dsn is required")
	}
	if c.Project.DefaultJoinMode == "" {
		c.Project.DefaultJoinMode = "free"
	}
	switch c.Project.DefaultJoinMode {
	case "free", "apply":
	default:
		return fmt.Errorf("project.default_join_mode must be free or apply")
	}
	return nil
}
