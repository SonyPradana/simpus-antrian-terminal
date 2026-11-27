// Package db builds the database/sql connection to the MySQL print server.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/SonyPradana/simpus-antrian-terminal/internal/config"

	mysql "github.com/go-sql-driver/mysql"
)

// DSN builds a safe DSN, so passwords do not need manual escaping.
// ParseTime is intentionally disabled: values such as '0000-00-00' must be
// read as strings so they can be validated instead of causing a scan error.
func DSN(cfg config.Config) string {
	c := mysql.NewConfig()
	c.User = cfg.DBUser
	c.Passwd = cfg.DBPassword
	c.Net = "tcp"
	c.Addr = net.JoinHostPort(cfg.DBHost, strconv.Itoa(cfg.DBPort))
	c.DBName = cfg.DBName
	c.Timeout = 5 * time.Second
	c.ReadTimeout = 5 * time.Second
	c.WriteTimeout = 5 * time.Second
	c.ParseTime = false
	return c.FormatDSN()
}

// Open opens the connection and pings it so serve fails early when the
// print server database is unreachable.
func Open(cfg config.Config) (*sql.DB, error) {
	sqlDB, err := sql.Open("mysql", DSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("open database connection: %w", err)
	}

	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("connect to %s:%d/%s: %w", cfg.DBHost, cfg.DBPort, cfg.DBName, err)
	}

	return sqlDB, nil
}
