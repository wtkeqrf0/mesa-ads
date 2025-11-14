package configs

import "net/url"

// Postgres holds configuration for connecting to a PostgreSQL database. The
// Addr field is a full connection string accepted by pgxpool.New. RunMigrations
// enables automatic migration execution on startup. MaxConns and MinConns
// control the size of the connection pool.
type Postgres struct {
	// Addr is a PostgreSQL connection string. It should include the
	// sslmode parameter if required.
	Addr url.URL `env:"ADDRESS" envDefault:"postgres://postgres:password@localhost:5432/postgres?sslmode=disable"`
	// RunMigrations controls whether database migrations are executed on
	// startup. Only honoured by main.
	RunMigrations bool `env:"RUN_MIGRATIONS" envDefault:"false"`
}
