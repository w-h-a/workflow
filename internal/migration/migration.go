package migration

import (
	"github.com/w-h-a/workflow/internal/migration/clients/migrator"
	schemamanager "github.com/w-h-a/workflow/internal/migration/services/schema_manager"
)

func NewSchemaManager(m migrator.Migrator) *schemamanager.Service {
	s := schemamanager.New(m)

	return s
}
