package adapters

import (
	"database/sql"
	"fmt"
	"log"
	"realtimer/internal/config"
	"runtime"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
)

var mysqldb *sql.DB

func newMySQL(cfg config.DBConfig) (*sql.DB, error) {
	// Capture connection properties.
	address := fmt.Sprintf("%s:%s", cfg.Database.Host, strconv.Itoa(cfg.Database.Port))

	mysqlConfig := mysql.Config{
		User:                 cfg.Database.Username,
		Passwd:               cfg.Database.Password,
		Net:                  "tcp",
		Addr:                 address,
		DBName:               cfg.Database.Name,
		AllowNativePasswords: true,
	}

	fmt.Println("connect db: ", mysqlConfig.FormatDSN())

	// Get a database handle.
	var err error
	mysqldb, err = sql.Open("mysql", mysqlConfig.FormatDSN())
	if err != nil {
		return nil, err
	}

	fmt.Println("ping db")

	pingErr := mysqldb.Ping()
	if pingErr != nil {
		fmt.Println(pingErr.Error())

		return nil, err
	}

	fmt.Println("init plugin")

	err = initMySQLPlugin()
	if err != nil {
		return nil, err
	}

	fmt.Println("init trigger")

	err = initMySqlTriggers(cfg)
	if err != nil {
		return nil, err
	}

	return mysqldb, nil
}

func initMySqlTriggers(cfg config.DBConfig) error {
	rows, err := mysqldb.Query("SELECT trigger_name FROM information_schema.triggers WHERE trigger_name LIKE 'realtimer_trigger_%';")
	if err != nil {
		return err
	}
	defer rows.Close()

	/// check if all tiggers exist
	// Map to keep track of existing triggers by table name
	existingTriggers := make(map[string]string)

	// Process the query results to track existing triggers
	for rows.Next() {
		var triggerName, _ string

		if err := rows.Scan(&triggerName); err != nil {
			return err
		}

		parts := strings.Split(triggerName, "_")
		if len(parts) > 3 {
			tableName := strings.Join(parts[3:], "_")
			// trigger name: table name
			existingTriggers[triggerName] = tableName
		}
	}

	// Loop over tables in config and check if triggers need to be created
	for _, table := range cfg.Tables {
		for _, operation := range table.Operations {
			key := fmt.Sprintf("realtimer_trigger_%s_%s", strings.ToLower(operation), table.Name)

			_, exists := existingTriggers[key]
			if !exists {
				// Trigger does not exist, so create it
				err := createMySqlTriggerForTable(table.Name, operation, cfg)
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
			err := dropMySqlTrigger(triggerName, cfg.Database.Name)
			if err != nil {
				return fmt.Errorf("failed to drop trigger %s: %w", triggerName, err)
			}
		}
	}

	return nil
}

func dropMySqlTrigger(triggerName, dbName string) error {
	dropTriggerQuery := fmt.Sprintf("DROP TRIGGER IF EXISTS %s.%s", dbName, triggerName)

	_, err := mysqldb.Exec(dropTriggerQuery)
	if err != nil {
		return err
	}
	return nil
}

// +-----------+-----+-------------------------+----------+
// | name      | ret | dl                      | type     |
// +-----------+-----+-------------------------+----------+
// | http_post |   0 | realtimer_requester.dll | function |
// +-----------+-----+-------------------------+----------+

// +---------------+----------------------------+
// | Variable_name | Value                      |
// +---------------+----------------------------+
// | plugin_dir    | C:\xampp\mysql\lib\plugin\ |
// +---------------+----------------------------+

func initMySQLPlugin() error {
	// Query to show active plugins
	showFunctionQuery := `SELECT * FROM mysql.func WHERE name = 'http_post';`

	// Execute the query to retrieve active plugins
	rows, err := mysqldb.Query(showFunctionQuery)
	if err != nil {
		return fmt.Errorf("error finding function: %w", err)
	}
	defer rows.Close()

	var (
		name  string
		ret   string
		dll   string
		Ttype string
	)

	pluginFound := false
	// Loop through all the active plugins and check if the desired plugin is available
	for rows.Next() {
		err = rows.Scan(&name, &ret, &dll, &Ttype)
		if err != nil {
			return fmt.Errorf("error scanning plugin row: %w", err)
		}

		if name == "http_post" {
			pluginFound = true
			break
		}
	}

	if pluginFound {
		return nil
	}

	fmt.Println("setting up plugin")

	// Query to find the plugin directory
	pluginDirQuery := `SHOW VARIABLES LIKE 'plugin_dir'`

	// Execute the query to retrieve the plugin directory
	row := mysqldb.QueryRow(pluginDirQuery)

	var variableName string
	var pluginDir string

	// Scan the result into variableName and pluginDir
	err = row.Scan(&variableName, &pluginDir)
	if err != nil {
		return fmt.Errorf("error finding plugin directory: %w", err)
	}

	if variableName != "plugin_dir" {
		return fmt.Errorf("unexpected result for plugin directory: %s", variableName)
	}

	fmt.Println("installing plugin function")

	var ext string = "so"
	srcFile := "udf/build/realtimer_requester.so"
	if runtime.GOOS == "windows" {
		ext = "dll"
		srcFile = "udf/build/realtimer_requester.dll"
	}

	dstFile := fmt.Sprintf("%srealtimer_requester.%s", pluginDir, ext)

	// Call the copyFile function
	err = copyFile(srcFile, dstFile)
	if err != nil {
		return fmt.Errorf("moving plugin failed: %e", err)
	}

	pluginInstallQuery := fmt.Sprintf(`CREATE FUNCTION http_post RETURNS STRING SONAME 'realtimer_requester.%s';`, ext)
	mysqldb.QueryRow(pluginInstallQuery)

	// Plugin directory found successfully, return it
	return nil
}

func createMySqlTriggerForTable(tableName string, operation string, cfg config.DBConfig) error {
	// Define trigger names for INSERT, UPDATE, and DELETE operations

	columnsQuery := fmt.Sprintf(`
		SELECT COLUMN_NAME
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = '%s' AND TABLE_NAME = '%s';
	`, cfg.Database.Name, tableName)

	cols, err := mysqldb.Query(columnsQuery)
	if err != nil {
		return fmt.Errorf("error failed: %e", err)
	}
	defer cols.Close()

	var columns []string
	for cols.Next() {
		var column string
		if err := cols.Scan(&column); err != nil {
			log.Fatal(err)
		}
		columns = append(columns, column)
	}

	if operation == "INSERT" {

		var columnConcatenation []string
		for _, column := range columns {

			columnConcatenation = append(columnConcatenation, fmt.Sprintf("'%s: ', IFNULL(NEW.%s, 'NULL')", column, column))
		}

		triggerInsert := fmt.Sprintf("realtimer_trigger_insert_%s", tableName)
		// Create the INSERT trigger
		insertTriggerQuery := fmt.Sprintf(`
			CREATE TRIGGER %s AFTER INSERT ON %s.%s
			FOR EACH ROW
			BEGIN
				DECLARE row_data TEXT;

				SET row_data = CONCAT(%s);

				SELECT http_post( '%s:%s/api/db?table=%s&event=INSERT', 'text/plain', row_data ) INTO @x;
			END`,
			triggerInsert,
			cfg.Database.Name,
			tableName,
			strings.Join(columnConcatenation, ", ', ', "),
			cfg.Servers.HttpBaseUrl,
			strconv.Itoa(cfg.Servers.HTTPPort),
			tableName,
		)

		// Execute the INSERT trigger creation
		_, err := mysqldb.Exec(insertTriggerQuery)
		if err != nil {
			return fmt.Errorf("error creating insert trigger: %w", err)
		}

	} else if operation == "UPDATE" {

		var columnConcatenation []string
		for _, column := range columns {
			columnConcatenation = append(columnConcatenation, fmt.Sprintf("'%s: ', IFNULL(NEW.%s, 'NULL')", column, column))
		}

		triggerUpdate := fmt.Sprintf("realtimer_trigger_update_%s", tableName)
		// Create the UPDATE trigger
		updateTriggerQuery := fmt.Sprintf(`
			CREATE TRIGGER %s AFTER UPDATE ON %s.%s
			FOR EACH ROW
			BEGIN
				DECLARE row_data TEXT;	

				SET row_data = CONCAT(%s);

				SELECT http_post( '%s:%s/api/db?table=%s&event=UPDATE', 'text/plain', row_data ) INTO @x;
			END;`,
			triggerUpdate,
			cfg.Database.Name,
			tableName,
			strings.Join(columnConcatenation, ", ', ', "),
			cfg.Servers.HttpBaseUrl,
			strconv.Itoa(cfg.Servers.HTTPPort),
			tableName,
		)

		// Execute the UPDATE trigger creation
		_, err := mysqldb.Exec(updateTriggerQuery)
		if err != nil {
			return fmt.Errorf("error creating update trigger: %w", err)
		}

	} else if operation == "DELETE" {

		var columnConcatenation []string
		for _, column := range columns {
			columnConcatenation = append(columnConcatenation, fmt.Sprintf("'%s: ', IFNULL(OLD.%s, 'NULL')", column, column))
		}

		triggerDelete := fmt.Sprintf("realtimer_trigger_delete_%s", tableName)
		// Create the DELETE trigger
		deleteTriggerQuery := fmt.Sprintf(`
			CREATE TRIGGER %s AFTER DELETE ON %s.%s
			FOR EACH ROW
			BEGIN
				DECLARE row_data TEXT;	

				SET row_data = CONCAT(%s);

				SELECT http_post( '%s:%s/api/db?table=%s&event=DELETE', 'text/plain', row_data ) INTO @x;
			END;`,
			triggerDelete,
			cfg.Database.Name,
			tableName,
			strings.Join(columnConcatenation, ", ', ', "),
			cfg.Servers.HttpBaseUrl,
			strconv.Itoa(cfg.Servers.HTTPPort),
			tableName,
		)

		// Execute the DELETE trigger creation
		_, err := mysqldb.Exec(deleteTriggerQuery)
		if err != nil {
			return fmt.Errorf("error creating delete trigger: %w", err)
		}
	}

	return nil
}
