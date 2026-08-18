cat << 'INNER_EOF' >> internal/parser/validator.go

// TestConnection attempts to ping the provided MySQL DSN to verify connectivity.
func TestConnection(dsn string) error {
	if dsn == "" {
		dsn = "root:@tcp(127.0.0.1:3306)/"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Ping()
}
INNER_EOF
