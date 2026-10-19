package retention

import "testing"

// SetBatchSize makes Expire work in batches of n for the rest of the test.
func SetBatchSize(t testing.TB, n int) {
	old := batchSize
	batchSize = n
	t.Cleanup(func() { batchSize = old })
}
