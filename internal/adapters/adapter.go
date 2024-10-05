package adapters

import (
	"database/sql"
	"errors"
	"fmt"
	"realtimer/internal/config"
	"strings"
)

var db *sql.DB

func New(cfg config.DBConfig) error {
	if cfg.Database.Type == "mysql" {
		_, err := newMySQL(cfg)

		if err != nil {
			return err
		}

		return nil
	} else if cfg.Database.Type == "postgres" {
		_, err := newPostgresAdapter(cfg)

		if err != nil {
			return err
		}

		return nil
	} else {
		return errors.New("undefined database type")
	}
}

// Helper function to check if a table is in the config
func isTableInConfig(triggerName string, tables []config.Table) bool {
	for _, table := range tables {
		for _, operation := range table.Operations {
			tName := fmt.Sprintf("realtimer_trigger_%s_%s", strings.ToLower(operation), table.Name)
			if tName == triggerName {
				return true
			}
		}
	}
	return false
}
