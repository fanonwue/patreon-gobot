package db

import (
	"github.com/fanonwue/patreon-gobot/internal/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"os"
	"path/filepath"
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
			return nil
		}
		db = openedDb
	}

	return db
}

func CreateDatabase() {
	Db()
	migrate()
}
