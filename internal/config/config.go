package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mnafees/click/internal/db"
)

type File struct {
	Profiles map[string]Profile `json:"profiles"`
}

type Profile struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

func LoadProfile(name string) (db.Config, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return db.Config{}, false
	}
	data, err := os.ReadFile(filepath.Join(home, ".clickrc"))
	if err != nil {
		return db.Config{}, false
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return db.Config{}, false
	}
	p, ok := f.Profiles[name]
	if !ok {
		return db.Config{}, false
	}
	cfg := db.Config{
		Host:     p.Host,
		Port:     p.Port,
		User:     p.User,
		Password: p.Password,
		Database: p.Database,
	}
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == 0 {
		cfg.Port = 9000
	}
	if cfg.User == "" {
		cfg.User = "default"
	}
	if cfg.Database == "" {
		cfg.Database = "default"
	}
	return cfg, true
}
