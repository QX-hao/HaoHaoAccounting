package categories

type categoryRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type listQuery struct {
	Type string `form:"type" binding:"omitempty,oneof=income expense"`
}
