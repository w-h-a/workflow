package migrator

type Migrator interface {
	Migrate(opts ...MigrateOption) error
}
