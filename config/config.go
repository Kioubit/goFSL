package config

import (
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
	Accounts      []account `toml:"accounts"`
}

func ReadConfig(configFile string) error {
	if configFile == "" {
		return nil
	}
	_, err := toml.DecodeFile(configFile, &GlobalConfig)
	return err
}
