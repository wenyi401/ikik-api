package admin

// UpdatePointsRequest represents points update request.
type UpdatePointsRequest struct {
	Points    float64 `json:"points" binding:"required,gt=0"`
	Operation string  `json:"operation" binding:"required,oneof=set add subtract"`
	Notes     string  `json:"notes"`
}
