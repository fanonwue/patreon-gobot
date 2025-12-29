package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fanonwue/goutils/logging"
	"github.com/fanonwue/patreon-gobot/internal/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const latestSchemaVersion = 1

var db *gorm.DB

func Db() *gorm.DB {
	if db == nil {
		databasePath := os.Getenv(util.PrefixEnvVar("DATABASE_PATH"))
		if databasePath == "" {
			databasePath = "./data/main.db"
		}

		os.MkdirAll(filepath.Dir(databasePath), os.ModePerm)

		openedDb, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
		if err != nil {
			panic(fmt.Sprintf("error opening database: %s", err))
		}

		sqlDB, err := openedDb.DB()
		if err != nil {
			logging.Errorf("Error retrieving sql DB interface: %s", err)
		} else {
			// Fix SQlite "database is locked"
			sqlDB.SetMaxOpenConns(1)
		}

		db = openedDb
	}

	return db
}

func CreateDatabase() {
	Db()
	migrate()
}
