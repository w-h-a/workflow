package cmd

import (
	"github.com/urfave/cli/v2"
	"github.com/w-h-a/workflow/internal/migration"
	"github.com/w-h-a/workflow/internal/migration/clients/migrator"
	"github.com/w-h-a/workflow/internal/migration/clients/migrator/postgres"
)

func RunMigrations(ctx *cli.Context) error {
	// get migrator
	m := postgres.NewMigrator(
		migrator.WithLocation(ctx.String("location")),
	)

	// get service
	schemaManager := migration.NewSchemaManager(m)

	// use service
	return schemaManager.CreateSchema()
}
