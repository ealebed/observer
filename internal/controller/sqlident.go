package controller

import (
	"strings"

	pgx "github.com/jackc/pgx/v5"
)

// sanitizeTableIdent returns a safely-quoted identifier suitable for SQL
// (supports "schema.table"). Defaults to public.server.
func sanitizeTableIdent(name string) string {
	if name == "" {
		name = "public.server"
	}
	parts := strings.Split(name, ".")
	return pgx.Identifier(parts).Sanitize()
}
