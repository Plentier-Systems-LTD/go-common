// Package logging builds a project's structured JSON logger via zap.
package logging

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New builds a zap logger that emits structured JSON logs. In development
// mode timestamps are human-readable; in all other environments it uses
// zap's production JSON encoding. An unrecognized level falls back to
// info.
func New(env, level string) (*zap.Logger, error) {
	var cfg zap.Config
	if env == "development" {
		cfg = zap.NewDevelopmentConfig()
		cfg.Encoding = "json"
	} else {
		cfg = zap.NewProductionConfig()
	}

	lvl, err := zapcore.ParseLevel(strings.ToLower(level))
	if err != nil {
		lvl = zapcore.InfoLevel
	}
	cfg.Level = zap.NewAtomicLevelAt(lvl)

	return cfg.Build()
}
