package batch_test

import (
	"testing"

	"github.com/perun-network/go-perun/db"
	"github.com/perun-network/go-perun/db/database_test"
)

/*
	GenericBatchTest is to be called from the batch implementation tests.
*/
func GenericBatchTest(t *testing.T, database db.Database) {
	this := BatchTest{T: t, Batch: database.NewBatch()}

	dbtest := database_test.DatabaseTest{T: t, Database: database}

	dbtest.Put("1234", "1234 initial value")
	dbtest.Put("5678", "5678 initial value")

	this.Batch.Reset()
	// Batch must be empty after reset.
	this.MustHaveLength(0)
	this.MustHaveValueSize(0)
	// Test that deleting works on empty and full batches.
	this.MustDelete("1234")
	this.MustDelete("5678")
	this.Batch.Reset()
	// Put must work on empty and full batches.
	// New Put() must increase length.
	this.MustPut("1234", "Test Put() tracking.")
	this.MustHaveLength(1)
	this.MustHaveValueSize(20)
	// Test PutBytes() overwrite.
	this.MustPutBytes("1234", []byte("Test PutBytes() tracking."))
	this.MustHaveLength(1)
	this.MustHaveValueSize(25)
	// Overwrite Put() must keep length.
	this.MustPut("1234", "Test Put() tracking override.")
	this.MustHaveLength(1)
	this.MustHaveValueSize(29)
	// New Delete() must increase length.
	this.MustDelete("5678")
	this.MustHaveLength(2)
	// Delete() of existing Put() must not change length.
	this.MustDelete("1234")
	this.MustHaveLength(2)
	this.MustHaveValueSize(0)
	// Put() of existing Delete() must not change length.
	this.MustPut("5678", "ghjk")
	this.MustHaveLength(2)
	this.MustHaveValueSize(4)

	this.MustApply()

	dbtest.MustNotHave("1234")
	dbtest.MustGetEqual("5678", "ghjk")

	return
}

type BatchTest struct {
	*testing.T
	Batch db.Batch
}

func (this *BatchTest) MustPut(key, value string) {
	if err := this.Batch.Put(key, value); err != nil {
		this.Fatalf("Put(): Failed to put ['%s'] = '%s': %v.\n", key, value, err)
	}
	return
}

func (this *BatchTest) MustPutBytes(key string, value []byte) {
	if err := this.Batch.PutBytes(key, value); err != nil {
		this.Fatalf("PutBytes(): Failed to put ['%s'] = '%v': %v\n", key, value, err)
	}
	return
}

func (this *BatchTest) MustDelete(key string) {
	if err := this.Batch.Delete(key); err != nil {
		this.Fatalf("Put(): Failed to delete ['%s']: %v.\n", key, err)
	}
	return
}

func (this *BatchTest) MustHaveLength(len uint) {
	if actual_len := this.Batch.Len(); actual_len != len {
		this.Errorf("Len(): Batch has %u elements, expected %u.\n", actual_len, len)
	}
	return
}

func (this *BatchTest) MustHaveValueSize(size uint) {
	if actual_size := this.Batch.ValueSize(); actual_size != size {
		this.Errorf("ValueSize(): Batch has size %u, expected %u.\n", actual_size, size)
	}
	return
}

func (this *BatchTest) MustApply() {
	if err := this.Batch.Apply(); err != nil {
		this.Errorf("Apply(): Failed with reason %v.\n", err)
	}
}
