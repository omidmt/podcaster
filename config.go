package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"
)

type Config struct {
	MaxDownloads int      `yaml:"max_downloads"`
	DownloadDir  string   `yaml:"download_dir"`
	DatabaseFile string   `yaml:"database_file"`
	LogTime      bool     `yaml:"log_time"`
	LogLevel     LogLevel `yaml:"log_level"`
}

type LogLevel zapcore.Level

var configPath = flag.String("config", "/etc/podcaster/config.yaml", "Path of the config file")

func (s *LogLevel) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var lvl string
	if err := unmarshal(&lvl); err != nil {
		return err
	}

	level := zap.InfoLevel // default
	switch strings.ToLower(lvl) {
	case "debug":
		level = zap.DebugLevel
	case "info":
		level = zap.InfoLevel
	case "warn":
		level = zap.WarnLevel
	case "error":
		level = zap.ErrorLevel

	default:
		fmt.Printf("unknown log level in config (%s), fallback to info level\n", lvl)
	}

	*s = LogLevel(level)
	return nil
}

func LoadConfig() *Config {
	var config = Config{
		MaxDownloads: 10,
		DownloadDir:  "./downloads",
		DatabaseFile: "./podcasts.db",
		LogTime:      true,
		LogLevel:     LogLevel(zap.DebugLevel),
	}

	bytes, err := os.ReadFile(*configPath)
	if err != nil {
		fmt.Println("reading config file failed, using defaults: ", err)
		return &config
	}

	err = yaml.Unmarshal(bytes, &config)
	if err != nil {
		fmt.Println("parsing config file failed: ", err)
		return &config
	}

	return &config
}
