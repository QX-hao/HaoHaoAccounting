package categories

type categoryRequest struct {
	Name string `json:"name" binding:"required,min=1"`
	Type string `json:"type" binding:"required,oneof=income expense"`
}

type listQuery struct {
	Type string `form:"type" binding:"omitempty,oneof=income expense"`
}
