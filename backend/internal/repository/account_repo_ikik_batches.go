package repository

const accountRepositoryIDBatchSize = 10000

func forEachAccountRepositoryIDBatch(ids []int64, fn func([]int64) error) error {
	seen := make(map[int64]struct{}, len(ids))
	uniqueIDs := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	for start := 0; start < len(uniqueIDs); start += accountRepositoryIDBatchSize {
		end := start + accountRepositoryIDBatchSize
		if end > len(uniqueIDs) {
			end = len(uniqueIDs)
		}
		if err := fn(uniqueIDs[start:end]); err != nil {
			return err
		}
	}
	return nil
}
