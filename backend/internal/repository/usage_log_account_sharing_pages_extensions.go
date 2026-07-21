package repository

func accountSharingPages(total int64, pageSize int) int {
	if pageSize < 1 {
		pageSize = 20
	}
	if total <= 0 {
		return 1
	}
	return int((total + int64(pageSize) - 1) / int64(pageSize))
}
