package migrations

import "embed"

// FS embeds SQL migration files stored in this directory. The
// golang-migrate library will read these files via the iofs driver when
// applying migrations.
//
//go:embed *.sql
var FS embed.FS

const Version = 1
