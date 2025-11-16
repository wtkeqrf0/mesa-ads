package db

import (
	"errors"
	"mesa-ads/migrations"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Migrate applies all up migrations found in the
func Migrate(addr string) error {
	driver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}
	defer driver.Close()

	mg, err := migrate.NewWithSourceInstance("iofs", driver, addr)
	if err != nil {
		return err
	}
	defer mg.Close()

	_, dirty, err := mg.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return err
	}

	if dirty {
		return errors.New("database is in dirty state")
	}

	if err = mg.Migrate(migrations.Version); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}
