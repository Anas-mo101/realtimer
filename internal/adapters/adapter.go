package adapters

import (
	"errors"
	"fmt"
	"io"
	"os"
	"realtimer/internal/config"
	"realtimer/internal/pubsub"
	"strings"
)

func New(cfg config.DBConfig, pubsub *pubsub.SubscriptionManager) error {
	if cfg.Database.Type == "mysql" {
		_, err := newMySQL(cfg)

		if err != nil {
			return err
		}

		return nil
	} else if cfg.Database.Type == "postgres" {
		_, err := newPostgress(cfg, pubsub)

		if err != nil {
			return err
		}

		return nil
	} else {
		return errors.New("undefined database type")
	}
}

// copyFile copies a file from src to dst.
func copyFile(srcFile string, dstFile string) error {
	// Open the source file
	src, err := os.Open(srcFile)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	// Create the destination file
	dst, err := os.Create(dstFile)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Copy the contents from source to destination
	_, err = io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
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
