package migrate

import (
	"fmt"

	"github.com/dbmove/dbmove/backend/internal/model"
	"gorm.io/gorm"
)

// Auto runs GORM AutoMigrate for all platform models.
func Auto(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.Connection{},
		&model.MigrationTask{},
		&model.MigrationLog{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	return nil
}
