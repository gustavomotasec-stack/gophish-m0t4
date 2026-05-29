package db

import (
	"os"
	"sync"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DB   *gorm.DB
	once sync.Once
)

// Connect returns the singleton DB connection
func Connect() *gorm.DB {
	once.Do(func() {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			panic("DATABASE_URL environment variable not set")
		}
		var err error
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			panic("failed to connect to database: " + err.Error())
		}
		migrate(DB)
	})
	return DB
}

func migrate(db *gorm.DB) {
	// Import models here to avoid circular imports
	// AutoMigrate is called from models package
}
