package memorydb

import (
	"testing"

	"github.com/perun-network/go-perun/db/database_test"
)

func TestDatabase(t *testing.T) {
	t.Run("Generic Database test", func(t *testing.T) {
		database_test.GenericDatabaseTest(t, NewDatabase())
	})
	return
}
