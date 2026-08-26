package sqliteutil

import "database/sql"

func CountRows(dbPath string, journalOff bool, query string) (int, error) {
	var count int
	err := QuerySQLite(dbPath, journalOff, query, func(rows *sql.Rows) error {
		return rows.Scan(&count)
	})
	return count, err
}

func QueryRows[T any](dbPath string, journalOff bool, query string, scanRow func(*sql.Rows) (T, error)) ([]T, error) {
	var results []T
	err := QuerySQLite(dbPath, journalOff, query, func(rows *sql.Rows) error {
		row, err := scanRow(rows)
		if err != nil {
			return err
		}
		results = append(results, row)
		return nil
	})
	return results, err
}
