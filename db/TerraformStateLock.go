package db

import "time"

const TerraformMaxStateBytes = 32 << 20

// Field names follow the Terraform/OpenTofu HTTP backend protocol.
type TerraformStateLock struct {
	ID        string
	Operation string
	Info      string
	Who       string
	Version   string
	Created   time.Time
	Path      string
}

type TerraformLockError struct{ Lock TerraformStateLock }

func (e *TerraformLockError) Error() string {
	return "Terraform state is locked or the lock ID is no longer valid"
}
