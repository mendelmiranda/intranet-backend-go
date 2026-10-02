package platform

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/microsoft/go-mssqldb"
)

// Open abre um pool database/sql. O pool conecta de forma preguiçosa, então o
// serviço sobe mesmo com o banco indisponível e falha apenas nas requisições.
func Open(driver, dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DSN do driver %s não informado", driver)
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}
