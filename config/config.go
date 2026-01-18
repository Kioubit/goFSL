package config

import (
	"log/slog"

	"github.com/BurntSushi/toml"
)

var DataDir = "data/"

var GlobalConfig = config{}

type account struct {
	Username string `toml:"username"`
	Password string `toml:"password"`
}

type config struct {
	MaxFileSizeMb uint64    `toml:"max_file_mb"`
	AccountList   []account `toml:"account"`
}

func ReadConfig(configFile string) error {
	if configFile == "" {
		slog.Warn("No config file specified, using defaults.")
		return nil
	}
	_, err := toml.DecodeFile(configFile, &GlobalConfig)
	if err != nil {
		return err
	}
	slog.Info("Accounts registered", "count", len(GlobalConfig.AccountList))
	return nil
}
