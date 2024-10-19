package adapters

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"realtimer/internal/config"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
)

func newPostgresAdapter(cfg config.DBConfig) (*sql.DB, error) {

	host := fmt.Sprintf("%s:%s", cfg.Database.Host, strconv.Itoa(cfg.Database.Port))

	dsn := url.URL{
		Scheme: "postgres",
		Host:   host,
		User: url.UserPassword(
			cfg.Database.Username,
			cfg.Database.Password,
		),
		Path: cfg.Database.Name,
	}

	q := dsn.Query()
	q.Add("sslmode", "disable")
	dsn.RawQuery = q.Encode()

	var err error
	db, err = sql.Open("postgres", dsn.String())
	if err != nil {
		return nil, err
	}

	if !cfg.Servers.IsRemote {
		exist, perr := doesPostgresExtentionExist()
		if exist {
			return nil, perr
		}
	} else {
		err = initPGPlugin(cfg)
		if err != nil {
			return nil, err
		}
	}

	err = initPostgresTrigger(cfg)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func doesPostgresExtentionExist() (bool, error) {
	var extname string
	err := db.QueryRow("SELECT extname FROM pg_extension WHERE extname = 'http';").Scan(&extname)
	fmt.Println("extention: ", extname)
	if err != nil {
		return false, err
	}

	if extname != "http" {
		return false, fmt.Errorf("postgres http extention does not exist")
	}

	return true, nil
}

func initPGPlugin(cfg config.DBConfig) error {
	exist, _ := doesPostgresExtentionExist()
	if exist {
		return nil
	}

	var version string
	err := db.QueryRow("SHOW server_version;").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to get PostgreSQL version: %w", err)
	}

	if cfg.Database.Os == "windows" {
		extentionUrl := fmt.Sprintf("https://www.postgresonline.com/downloads/pg%shttp_w64.zip", version)

		// Define the file paths
		downloadPath := "pg_extension.zip"
		extractPath := "pg_extension"

		// Download the binary zip file
		err := downloadFile(downloadPath, extentionUrl)
		if err != nil {
			return fmt.Errorf("failed to download extension: %w", err)
		}

		// Unzip the downloaded file
		err = unzip(downloadPath, extractPath)
		if err != nil {
			return fmt.Errorf("failed to unzip extension: %w", err)
		}

		var postgresExtDir string
		err = db.QueryRow("SHOW (pg_config --sharedir)/extension;").Scan(&postgresExtDir)
		if err != nil {
			return fmt.Errorf("failed to get PostgreSQL version: %w", err)
		}

		err = filepath.Walk(extractPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				dest := filepath.Join(postgresExtDir, info.Name())
				err := os.Rename(path, dest)
				if err != nil {
					return fmt.Errorf("failed to copy extension file: %w", err)
				}
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to install extension: %w", err)
		}

	} else {
		// Linux/Debian
		cmd := exec.Command("sudo", "apt", "install", "-y", fmt.Sprintf("postgresql-%s-http", version))
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to install PostgreSQL HTTP extension: %w", err)
		}
	}

	initPgExtention := "CREATE EXTENSION IF NOT EXISTS http"
	_, err = db.Exec(initPgExtention)
	if err != nil {
		return fmt.Errorf("error creating update trigger: %w", err)
	}

	return nil
}

func downloadFile(filepath string, url string) error {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Download the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		// Ensure the path is inside the destination directory to prevent ZipSlip attacks
		if rel, err := filepath.Rel(dest, fpath); err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			err := os.MkdirAll(fpath, os.ModePerm)
			if err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

func initPostgresTrigger(cfg config.DBConfig) error {
	initFunctionQuery := fmt.Sprintf(
		`CREATE OR REPLACE FUNCTION realtimer_trigger(table TEXT, event TEXT, row_data TEXT) RETURNS TRIGGER AS $$
		BEGIN
			SELECT * http_post('%s:%s/api/db?table=$1&event=$2', $3, 'text/plain')
		END;
		$$ LANGUAGE plpgsql;`,
		cfg.Servers.HttpBaseUrl,
		strconv.Itoa(cfg.Servers.HTTPPort),
	)

	_, err := db.Exec(initFunctionQuery)
	if err != nil {
		return err
	}

	rows, err := db.Query("SELECT trigger_name FROM information_schema.triggers WHERE trigger_name LIKE 'realtimer_trigger_%';")
	if err != nil {
		return err
	}
	defer rows.Close()

	existingTriggers := make(map[string]string)
	for rows.Next() {
		var triggerName, tableName string

		if err := rows.Scan(&tableName, &triggerName); err != nil {
			return err
		}

		existingTriggers[triggerName] = tableName
	}

	// Loop over tables in config and check if triggers need to be created
	for _, table := range cfg.Tables {
		for _, operation := range table.Operations {
			key := fmt.Sprintf("realtimer_trigger_%s_%s", strings.ToLower(operation), table.Name)

			_, exists := existingTriggers[key]
			if !exists {
				// Trigger does not exist, so create it
				err := createPostgresTrigger(table.Name, operation)
				if err != nil {
					return fmt.Errorf("failed to create trigger for table %s: %w", table.Name, err)
				}
			}
		}
	}

	// Loop over existing triggers and drop those not in the current config
	for triggerName := range existingTriggers {
		if !isTableInConfig(triggerName, cfg.Tables) {
			// Trigger exists but is not in the current config, so drop it
			err := dropPostgresTrigger(triggerName, cfg.Database.Name)
			if err != nil {
				return fmt.Errorf("failed to drop trigger %s: %w", triggerName, err)
			}
		}
	}

	return nil
}

func dropPostgresTrigger(triggerName string, tableName string) error {
	dropTriggerQuery := fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s", triggerName, tableName)

	_, err := db.Exec(dropTriggerQuery)
	if err != nil {
		return err
	}
	return nil
}

func createPostgresTrigger(tableName string, operation string) error {
	columnsQuery := fmt.Sprintf(`
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = '%s';
	`, tableName)

	cols, err := db.Query(columnsQuery)
	if err != nil {
		return fmt.Errorf("error failed: %e", err)
	}
	defer cols.Close()

	var columns []string
	for cols.Next() {
		var column string
		if err := cols.Scan(&column); err != nil {
			return err
		}
		columns = append(columns, column)
	}

	var columnConcatenation []string
	for _, column := range columns {
		columnConcatenation = append(columnConcatenation, fmt.Sprintf("'%s: ', COALESCE(NEW.%s, 'NULL')", column, column))
	}

	initTriggerQuery := fmt.Sprintf(
		`CREATE OR REPLACE TRIGGER realtimer_trigger_%s_%s
		AFTER %s ON %s
		FOR EACH ROW EXECUTE FUNCTION realtimer_trigger('%s', '%s', '%s');`,
		strings.ToLower(operation),
		tableName,
		operation,
		tableName,
		tableName,
		operation,
		strings.Join(columnConcatenation, ", ', ', "),
	)

	_, err = db.Exec(initTriggerQuery)
	if err != nil {
		return err
	}

	return nil
}
