package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"go-api-boilerplate/config"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB holds the database connections
type DB struct {
	Write   *gorm.DB
	Read    *gorm.DB
	MongoDB *mongo.Database
}

var db *DB

// Connect establishes database connections based on configuration
func Connect(cfg *config.Config) (*DB, error) {
	var err error
	db = &DB{}

	// Setup logger
	logConfig := setupLogger(cfg)

	switch cfg.Database.Driver {
	case "postgres":
		db.Write, db.Read, err = connectPostgres(cfg, logConfig)
	case "mysql":
		db.Write, db.Read, err = connectMySQL(cfg, logConfig)
	case "sqlite":
		db.Write, db.Read, err = connectSQLite(cfg, logConfig)
	case "sqlserver":
		db.Write, db.Read, err = connectSQLServer(cfg, logConfig)
	case "mongodb":
		db.MongoDB, err = connectMongoDB(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
		}
		// Create dummy GORM connections for compatibility
		db.Write, db.Read = createDummyGORMConnections()
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Database.Driver)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool for SQL databases
	if cfg.Database.Driver != "mongodb" {
		configureConnectionPool(db.Write, cfg)
		if db.Read != db.Write {
			configureConnectionPool(db.Read, cfg)
		}
	}

	return db, nil
}

// setupLogger configures GORM logger
func setupLogger(cfg *config.Config) logger.Interface {
	logLevel := logger.Silent
	if cfg.IsDebug() {
		logLevel = logger.Info
	}

	return logger.New(
		log.New(log.Writer(), "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)
}

// connectPostgres establishes PostgreSQL connections
func connectPostgres(cfg *config.Config, logConfig logger.Interface) (*gorm.DB, *gorm.DB, error) {
	// Write connection
	writeDSN := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.Port,
		cfg.Database.SSLMode,
	)

	writeDB, err := gorm.Open(postgres.Open(writeDSN), &gorm.Config{
		Logger: logConfig,
	})
	if err != nil {
		return nil, nil, err
	}

	// Read connection
	var readDB *gorm.DB
	if cfg.Database.ReadHost != "" {
		readDSN := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
			cfg.Database.ReadHost,
			cfg.Database.ReadUser,
			cfg.Database.ReadPassword,
			cfg.Database.Name,
			cfg.Database.ReadPort,
			cfg.Database.SSLMode,
		)
		readDB, err = gorm.Open(postgres.Open(readDSN), &gorm.Config{
			Logger: logConfig,
		})
		if err != nil {
			return nil, nil, err
		}
	} else {
		readDB = writeDB
	}

	return writeDB, readDB, nil
}

// connectMySQL establishes MySQL connections
func connectMySQL(cfg *config.Config, logConfig logger.Interface) (*gorm.DB, *gorm.DB, error) {
	// Write connection
	writeDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
	)

	writeDB, err := gorm.Open(mysql.Open(writeDSN), &gorm.Config{
		Logger: logConfig,
	})
	if err != nil {
		return nil, nil, err
	}

	// Read connection
	var readDB *gorm.DB
	if cfg.Database.ReadHost != "" {
		readDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.Database.ReadUser,
			cfg.Database.ReadPassword,
			cfg.Database.ReadHost,
			cfg.Database.ReadPort,
			cfg.Database.Name,
		)
		readDB, err = gorm.Open(mysql.Open(readDSN), &gorm.Config{
			Logger: logConfig,
		})
		if err != nil {
			return nil, nil, err
		}
	} else {
		readDB = writeDB
	}

	return writeDB, readDB, nil
}

// connectSQLite establishes SQLite connection
func connectSQLite(cfg *config.Config, logConfig logger.Interface) (*gorm.DB, *gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(cfg.Database.Name), &gorm.Config{
		Logger: logConfig,
	})
	if err != nil {
		return nil, nil, err
	}

	// SQLite doesn't support read replicas, so both connections point to the same database
	return db, db, nil
}

// connectSQLServer establishes SQL Server connections
func connectSQLServer(cfg *config.Config, logConfig logger.Interface) (*gorm.DB, *gorm.DB, error) {
	// Write connection
	writeDSN := fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
	)

	writeDB, err := gorm.Open(sqlserver.Open(writeDSN), &gorm.Config{
		Logger: logConfig,
	})
	if err != nil {
		return nil, nil, err
	}

	// Read connection
	var readDB *gorm.DB
	if cfg.Database.ReadHost != "" {
		readDSN := fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s",
			cfg.Database.ReadUser,
			cfg.Database.ReadPassword,
			cfg.Database.ReadHost,
			cfg.Database.ReadPort,
			cfg.Database.Name,
		)
		readDB, err = gorm.Open(sqlserver.Open(readDSN), &gorm.Config{
			Logger: logConfig,
		})
		if err != nil {
			return nil, nil, err
		}
	} else {
		readDB = writeDB
	}

	return writeDB, readDB, nil
}

// connectMongoDB establishes MongoDB connection
func connectMongoDB(cfg *config.Config) (*mongo.Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.MongoDB.ConnectTimeout)
	defer cancel()

	clientOptions := options.Client().
		ApplyURI(cfg.MongoDB.URI).
		SetMaxPoolSize(cfg.MongoDB.MaxPoolSize)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Ping the database to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return client.Database(cfg.MongoDB.Database), nil
}

// createDummyGORMConnections creates dummy GORM connections for MongoDB compatibility
func createDummyGORMConnections() (*gorm.DB, *gorm.DB) {
	// Create in-memory SQLite database for compatibility
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	return db, db
}

// configureConnectionPool configures the connection pool for GORM
func configureConnectionPool(db *gorm.DB, cfg *config.Config) {
	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("Failed to get database connection: %v", err)
		return
	}

	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.Database.ConnMaxIdleTime)
}

// GetDB returns the database instance
func GetDB() *DB {
	if db == nil {
		log.Fatal("Database not initialized. Call Connect() first")
	}
	return db
}

// Close closes all database connections
func Close() error {
	if db == nil {
		return nil
	}

	// Close SQL databases
	if db.Write != nil {
		sqlDB, err := db.Write.DB()
		if err == nil {
			sqlDB.Close()
		}
	}

	if db.Read != nil && db.Read != db.Write {
		sqlDB, err := db.Read.DB()
		if err == nil {
			sqlDB.Close()
		}
	}

	// Close MongoDB
	if db.MongoDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := db.MongoDB.Client().Disconnect(ctx); err != nil {
			return fmt.Errorf("failed to disconnect MongoDB: %w", err)
		}
	}

	return nil
}

// HealthCheck performs a health check on the database connections
func HealthCheck() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Check SQL database
	if db.Write != nil {
		sqlDB, err := db.Write.DB()
		if err != nil {
			return fmt.Errorf("failed to get write database: %w", err)
		}
		if err := sqlDB.Ping(); err != nil {
			return fmt.Errorf("write database ping failed: %w", err)
		}
	}

	if db.Read != nil && db.Read != db.Write {
		sqlDB, err := db.Read.DB()
		if err != nil {
			return fmt.Errorf("failed to get read database: %w", err)
		}
		if err := sqlDB.Ping(); err != nil {
			return fmt.Errorf("read database ping failed: %w", err)
		}
	}

	// Check MongoDB
	if db.MongoDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := db.MongoDB.Client().Ping(ctx, nil); err != nil {
			return fmt.Errorf("MongoDB ping failed: %w", err)
		}
	}

	return nil
}

// Transaction executes a function within a database transaction
func Transaction(fn func(*gorm.DB) error) error {
	return db.Write.Transaction(fn)
}

// IsMongoDB returns true if using MongoDB
func IsMongoDB() bool {
	cfg := config.Get()
	return cfg.Database.Driver == "mongodb"
}
