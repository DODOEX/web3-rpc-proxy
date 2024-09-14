package database

import (
	"context"
	"flag"
	"time"

	"github.com/DODOEX/web3rpcproxy/internal/app/database/schema"
	"github.com/DODOEX/web3rpcproxy/internal/app/database/seeds"
	"github.com/DODOEX/web3rpcproxy/utils/config"
	"github.com/rs/zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setup database with gorm
type Database struct {
	logger zerolog.Logger
	config *config.Conf
	DB     *gorm.DB
}

type Seeder interface {
	Seed(*gorm.DB) error
	Count(*gorm.DB) (int, error)
}

func NewDatabase(config *config.Conf, logger zerolog.Logger) *Database {
	db := &Database{
		config: config,
		logger: logger,
	}

	return db
}

// connect database
func (d *Database) Connect(ctx context.Context) error {
	if d.DB != nil {
		d.logger.Info().Msg("The database is already connected!")
		return nil
	}

	config := &gorm.Config{
		SkipDefaultTransaction:                   true,
		PrepareStmt:                              true,
		DisableForeignKeyConstraintWhenMigrating: true,
	}
	d.config.Unmarshal("database.gorm", config)
	db, err := gorm.Open(postgres.Open(d.config.String("database.postgres.dsn")), config)
	if err != nil {
		return err
	}

	d.DB = db

	_db, _ := db.DB()
	_db.SetConnMaxIdleTime(d.config.Duration("database.gorm.conn-max-idle-time", 30*time.Second))
	if d.config.Exists("database.gorm.conn-max-lifetime") {
		_db.SetConnMaxLifetime(d.config.Duration("database.gorm.conn-max-lifetime"))
	}
	if d.config.Exists("database.gorm.max-idle-conns") {
		_db.SetMaxIdleConns(d.config.Int("database.gorm.max-idle-conns"))
	}
	if d.config.Exists("database.gorm.max-open-conns") {
		_db.SetMaxOpenConns(d.config.Int("database.gorm.max-open-conns"))
	}

	// read flag -migrate to migrate the database
	migrate := flag.Bool("migrate", false, "migrate the database")
	// read flag -seed to seed the database
	seeder := flag.Bool("seed", false, "seed the database")
	flag.Parse()

	if *migrate || d.config.Bool("database.gorm.migrate", false) {
		d.logger.Info().Msg("- Migrating the database...")
		d.MigrateModels(ctx)
	}
	if *seeder || d.config.Bool("database.gorm.seed", false) {
		d.logger.Info().Msg("- Seeding the database...")
		d.SeedModels(ctx)
	}

	return nil
}

// shutdown database
func (d *Database) Close() error {
	if d != nil || d.DB == nil {
		return nil
	}
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.Close(); err != nil {
		return err
	}
	return nil
}

// list of models for migration
func Models() []interface{} {
	return []interface{}{
		schema.Tenant{},
	}
}

// migrate models
func (d *Database) MigrateModels(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, d.config.Duration("database.gorm.migrate-timeout", 10*time.Minute))
	defer cancel()
	if err := d.DB.WithContext(ctx).AutoMigrate(
		Models()...,
	); err != nil {
		d.logger.Error().Err(err).Msg("An unknown error occurred when to migrate the database!")
	}
}

// list of models for migration
func Seeders() []Seeder {
	return []Seeder{
		seeds.TenantSeeder{},
	}
}

// seed data
func (d *Database) SeedModels(ctx context.Context) {
	seeders := Seeders()
	for _, seed := range seeders {
		db := d.DB.WithContext(ctx)
		count, err := seed.Count(db)
		if err != nil {
			d.logger.Error().Err(err).Msg("An unknown error occurred when to seed the database!")
		}

		if count == 0 {
			if err := seed.Seed(db); err != nil {
				d.logger.Error().Err(err).Msg("An unknown error occurred when to seed the database!")
			}

			d.logger.Info().Msg("Seeded the database succesfully!")
		} else {
			d.logger.Info().Msg("Database is already seeded!")
		}
	}

	d.logger.Info().Msg("Seeded the database succesfully!")
}
