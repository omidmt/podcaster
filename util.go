package main

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var AppConfig *Config = LoadConfig()

func BuildLogger(name string) *zap.Logger {
	atomicLevel := zap.NewAtomicLevelAt(zap.DebugLevel)

	// Use a config to create the logger. Set the initial level to info.
	config := zap.Config{
		Level:       atomicLevel,
		Development: false,
		Encoding:    "console", // "json",
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:    "message",
			LevelKey:      "level",
			TimeKey:       "time",
			NameKey:       "logging",
			CallerKey:     "caller",
			StacktraceKey: "stacktrace",
			LineEnding:    zapcore.DefaultLineEnding,
			// EncodeLevel:   zapcore.LowercaseLevelEncoder,
			// EncodeTime:    zapcore.EpochTimeEncoder,
			EncodeLevel:    zapcore.CapitalLevelEncoder, // capitalized level
			EncodeTime:     zapcore.ISO8601TimeEncoder,  // ISO8601 UTC time format
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	if AppConfig != nil && !AppConfig.LogTime {
		config.EncoderConfig.TimeKey = ""
	}

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}

	// TODO: remove or move to another function
	// atomicLevel.SetLevel(zapcore.Level(AppConfig.LogLevel))

	return logger.Named(name)
}
