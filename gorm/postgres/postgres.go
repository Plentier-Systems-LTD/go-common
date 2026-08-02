// Package postgres opens the shared Postgres connection a project's domain
// packages use for every query, via github.com/ochom/gutils/sqlr. sqlr
// holds the connection as a package-level singleton, so once Init
// succeeds, repositories reach it directly (sqlr.GORM(), sqlr.FindOne[T],
// ...) without threading a *gorm.DB through every constructor.
package postgres

import (
	"fmt"

	"github.com/ochom/gutils/sqlr"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Config configures Init.
type Config struct {
	DSN string
	// Debug enables verbose SQL logging (gorm logger.Info); queries are
	// silent otherwise.
	Debug bool
}

// Init opens and pings the Postgres connection sqlr uses for every query
// in the app, returning the underlying *gorm.DB for callers that need it
// directly (auto-migrations, go-common packages taking *gorm.DB).
func Init(cfg Config) (*gorm.DB, error) {
	logLevel := gormlogger.Silent
	if cfg.Debug {
		logLevel = gormlogger.Info
	}

	if err := sqlr.Init(&sqlr.Config{
		Conn:     gormpostgres.Open(cfg.DSN),
		LogLevel: logLevel,
	}); err != nil {
		return nil, fmt.Errorf("postgres: failed to connect: %w", err)
	}

	db := sqlr.GORM()

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("postgres: failed to get underlying sql.DB: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("postgres: failed to ping: %w", err)
	}

	return db, nil
}
