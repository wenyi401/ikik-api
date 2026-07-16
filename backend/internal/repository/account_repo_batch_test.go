package repository

import "testing"

func TestForEachAccountRepositoryIDBatch(t *testing.T) {
	ids := make([]int64, 0, accountRepositoryIDBatchSize+4)
	ids = append(ids, 0, -1, 1, 1)
	for i := int64(2); i <= int64(accountRepositoryIDBatchSize+2); i++ {
		ids = append(ids, i)
	}

	var batches [][]int64
	if err := forEachAccountRepositoryIDBatch(ids, func(batch []int64) error {
		copied := append([]int64(nil), batch...)
		batches = append(batches, copied)
		return nil
	}); err != nil {
		t.Fatalf("forEachAccountRepositoryIDBatch returned error: %v", err)
	}

	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}
	if got := len(batches[0]); got != accountRepositoryIDBatchSize {
		t.Fatalf("first batch size = %d, want %d", got, accountRepositoryIDBatchSize)
	}
	if got := len(batches[1]); got != 2 {
		t.Fatalf("second batch size = %d, want 2", got)
	}
	if batches[0][0] != 1 {
		t.Fatalf("first id = %d, want 1", batches[0][0])
	}
	if batches[1][1] != int64(accountRepositoryIDBatchSize+2) {
		t.Fatalf("last id = %d, want %d", batches[1][1], accountRepositoryIDBatchSize+2)
	}
}
