package config

import (
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var config Config

// https://github.com/spf13/viper
func InitConfig() {
	viper.SetConfigName("netbootd")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/netbootd/")
	viper.AddConfigPath("$HOME/.config/netbootd/")
	viper.AddConfigPath(".")

	viper.SetDefault("store.path", "/var/lib/netbootd")

	viper.SetEnvPrefix("netbootd")
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err != nil {
		log.Debug().
			Err(err).
			Msg("error reading config file")
	}
}

// Read (or re-read) the config from external source
func Read() error {
	// https://github.com/spf13/viper#unmarshaling to struct
	return viper.Unmarshal(&config)
}

// Get copy of running config
func GetConfig() Config {
	return config
}

func Watch() {
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Info().
			Str("path", e.Name).
			Msg("Config file reloaded")
	})
}
