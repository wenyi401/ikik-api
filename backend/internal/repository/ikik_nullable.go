package repository

func nullablePtrInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
