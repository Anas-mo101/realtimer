package adapters

import (
	"context"
	"fmt"
	"net/url"
	"realtimer/internal/config"

	"github.com/jackc/pgx/v5"
)

var postgressdb *pgx.Conn

func newPostgress(cfg config.DBConfig) (*pgx.Conn, error) {
	ctx := context.Background()

	host := fmt.Sprintf("%s:%s", cfg.Database.Host, cfg.Database.Host)

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
	postgressdb, err = pgx.Connect(ctx, dsn.String())
	if err != nil {
		return nil, err
	}

	initPostgresTriggers(cfg)

	return postgressdb, nil
}

func initPostgresTriggers(cfg config.DBConfig) error {
	ctx := context.Background()

	// Query to check existing triggers
	rows, err := postgressdb.Query(ctx, fmt.Sprintf(`
        SELECT tgname, relname 
        FROM pg_trigger 
        JOIN pg_class ON pg_trigger.tgrelid = pg_class.oid 
        WHERE tgname LIKE 'realtimer_trigger%%' AND relnamespace IN 
        (SELECT oid FROM pg_namespace WHERE nspname = '%s');`, cfg.Database.Name))

	if err != nil {
		return err
	}
	defer rows.Close()

	// Map to keep track of existing triggers by table name
	existingTriggers := make(map[string]bool)

	// Process the query results to track existing triggers
	for rows.Next() {
		var triggerName, tableName string
		if err := rows.Scan(&triggerName, &tableName); err != nil {
			return err
		}

		// Mark the trigger as existing for the table
		existingTriggers[tableName] = true
	}

	// Loop over tables in config and check if triggers need to be created
	for _, table := range cfg.Tables {
		if !existingTriggers[table.Name] {
			// Trigger does not exist, so create it
			err := createPostgresTriggerForTable(table)
			if err != nil {
				return fmt.Errorf("failed to create trigger for table %s: %w", table.Name, err)
			}
		}
	}

	return nil
}

func createPostgresTriggerForTable(table config.Table) error {
	ctx := context.Background()

	// Loop over each operation specified in the config for the table
	for _, operation := range table.Operations {
		if operation == "INSERT" {
			triggerInsert := fmt.Sprintf("realtimer_trigger_insert_%s", table.Name)
			// Create the INSERT trigger
			insertTriggerQuery := fmt.Sprintf(`
                CREATE OR REPLACE FUNCTION realtime_notify_insert_%s() RETURNS TRIGGER AS $$
                BEGIN
                    PERFORM pg_notify('realtimer', json_build_object(
                        'table', TG_TABLE_NAME,
                        'operation', 'INSERT',
                        'new_row', row_to_json(NEW)
                    )::text);
                    RETURN NEW;
                END;
                $$ LANGUAGE plpgsql;

                CREATE TRIGGER %s
                AFTER INSERT ON %s
                FOR EACH ROW
                EXECUTE FUNCTION realtime_notify_insert_%s();`,
				table.Name, triggerInsert, table.Name, table.Name)

			// Execute the INSERT trigger creation
			_, err := postgressdb.Exec(ctx, insertTriggerQuery)
			if err != nil {
				return fmt.Errorf("error creating insert trigger: %w", err)
			}

		} else if operation == "UPDATE" {
			triggerUpdate := fmt.Sprintf("realtimer_trigger_update_%s", table.Name)
			// Create the UPDATE trigger
			updateTriggerQuery := fmt.Sprintf(`
                CREATE OR REPLACE FUNCTION realtime_notify_update_%s() RETURNS TRIGGER AS $$
                BEGIN
                    PERFORM pg_notify('realtimer', json_build_object(
                        'table', TG_TABLE_NAME,
                        'operation', 'UPDATE',
                        'new_row', row_to_json(NEW)
                    )::text);
                    RETURN NEW;
                END;
                $$ LANGUAGE plpgsql;

                CREATE TRIGGER %s
                AFTER UPDATE ON %s
                FOR EACH ROW
                EXECUTE FUNCTION realtime_notify_update_%s();`,
				table.Name, triggerUpdate, table.Name, table.Name)

			// Execute the UPDATE trigger creation
			_, err := postgressdb.Exec(ctx, updateTriggerQuery)
			if err != nil {
				return fmt.Errorf("error creating update trigger: %w", err)
			}

		} else if operation == "DELETE" {
			triggerDelete := fmt.Sprintf("realtimer_trigger_delete_%s", table.Name)
			// Create the DELETE trigger
			deleteTriggerQuery := fmt.Sprintf(`
                CREATE OR REPLACE FUNCTION realtime_notify_delete_%s() RETURNS TRIGGER AS $$
                BEGIN
                    PERFORM pg_notify('realtimer', json_build_object(
                        'table', TG_TABLE_NAME,
                        'operation', 'DELETE',
                        'old_row', row_to_json(OLD)
                    )::text);
                    RETURN OLD;
                END;
                $$ LANGUAGE plpgsql;

                CREATE TRIGGER %s
                AFTER DELETE ON %s
                FOR EACH ROW
                EXECUTE FUNCTION realtime_notify_delete_%s();`,
				table.Name, triggerDelete, table.Name, table.Name)

			// Execute the DELETE trigger creation
			_, err := postgressdb.Exec(ctx, deleteTriggerQuery)
			if err != nil {
				return fmt.Errorf("error creating delete trigger: %w", err)
			}
		}
	}

	return nil
}
