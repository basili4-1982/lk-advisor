// Package db embeds SQL migrations.
package db

import "embed"

// Migrations contains the ordered SQL migration files.
//
//go:embed migrations/*.sql
var Migrations embed.FS
