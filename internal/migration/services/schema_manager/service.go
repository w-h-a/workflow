package schemamanager

import "github.com/w-h-a/workflow/internal/migration/clients/migrator"

type Service struct {
	migrator migrator.Migrator
}

func (s *Service) CreateSchema() error {
	return s.migrator.Migrate()
}

func New(migrator migrator.Migrator) *Service {
	return &Service{
		migrator: migrator,
	}
}
