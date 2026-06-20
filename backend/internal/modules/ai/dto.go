package ai

type parseRequest struct {
	Text string `json:"text" binding:"required,min=1"`
}
