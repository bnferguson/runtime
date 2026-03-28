package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

const (
	defaultMysqlDB   = "mysql"
	defaultMysqlUser = "root"
)

// maxMysqlIdentLen is the maximum length of a MySQL username or database name.
const maxMysqlIdentLen = 32

func sanitizeIdentifier(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			result = append(result, c)
		} else if c >= 'A' && c <= 'Z' {
			result = append(result, c+32) // lowercase
		} else if c == '-' {
			result = append(result, '_')
		}
	}
	if len(result) == 0 {
		return "app"
	}
	if result[0] >= '0' && result[0] <= '9' {
		result = append([]byte{'a'}, result...)
	}
	if len(result) > maxMysqlIdentLen {
		result = result[:maxMysqlIdentLen]
	}
	return string(result)
}

// quoteIdentifier wraps a MySQL identifier in backticks, escaping any
// embedded backticks by doubling them.
func quoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func connectMysql(ctx context.Context, host string, port int, user, password, database string) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?tls=false", user, password, host, port, database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening mysql connection to %s:%d: %w", host, port, err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to mysql at %s:%d: %w", host, port, err)
	}

	return db, nil
}

func connectAsRoot(ctx context.Context, host string, password string) (*sql.DB, error) {
	return connectMysql(ctx, host, mysqlPort, defaultMysqlUser, password, defaultMysqlDB)
}

func createMysqlUser(ctx context.Context, db *sql.DB, username, password string) error {
	_, err := db.ExecContext(ctx,
		fmt.Sprintf("CREATE USER %s@'%%' IDENTIFIED BY '%s'",
			quoteIdentifier(username), strings.ReplaceAll(password, "'", "''")))
	if err != nil {
		return fmt.Errorf("creating user %s: %w", username, err)
	}
	return nil
}

func createMysqlDatabase(ctx context.Context, db *sql.DB, dbname, owner string) error {
	_, err := db.ExecContext(ctx,
		fmt.Sprintf("CREATE DATABASE %s", quoteIdentifier(dbname)))
	if err != nil {
		return fmt.Errorf("creating database %s: %w", dbname, err)
	}

	_, err = db.ExecContext(ctx,
		fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO %s@'%%'",
			quoteIdentifier(dbname), quoteIdentifier(owner)))
	if err != nil {
		return fmt.Errorf("granting privileges on %s to %s: %w", dbname, owner, err)
	}

	_, err = db.ExecContext(ctx, "FLUSH PRIVILEGES")
	if err != nil {
		return fmt.Errorf("flushing privileges: %w", err)
	}

	return nil
}

func dropMysqlDatabase(ctx context.Context, db *sql.DB, dbname string) error {
	_, err := db.ExecContext(ctx,
		fmt.Sprintf("DROP DATABASE IF EXISTS %s", quoteIdentifier(dbname)))
	if err != nil {
		return fmt.Errorf("dropping database %s: %w", dbname, err)
	}
	return nil
}

func dropMysqlUser(ctx context.Context, db *sql.DB, username string) error {
	_, err := db.ExecContext(ctx,
		fmt.Sprintf("DROP USER IF EXISTS %s@'%%'", quoteIdentifier(username)))
	if err != nil {
		return fmt.Errorf("dropping user %s: %w", username, err)
	}
	return nil
}

func terminateMysqlConnections(ctx context.Context, db *sql.DB, dbname string) error {
	rows, err := db.QueryContext(ctx,
		"SELECT ID FROM INFORMATION_SCHEMA.PROCESSLIST WHERE DB = ? AND ID != CONNECTION_ID()", dbname)
	if err != nil {
		return fmt.Errorf("listing connections to %s: %w", dbname, err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scanning process id: %w", err)
		}
		_, _ = db.ExecContext(ctx, fmt.Sprintf("KILL %d", id))
	}

	return rows.Err()
}
