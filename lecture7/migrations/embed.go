package migrations

import "embed"

// Files содержит SQL-миграции внутри скомпилированной программы.
//
//go:embed *.sql
var Files embed.FS
