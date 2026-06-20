package auth

import "github.com/QX-hao/HaoHaoAccounting/backend/internal/models"

type loginRequest struct {
	Username string `json:"username" binding:"required,min=1"`
	Password string `json:"password" binding:"required,min=1"`
}

type currentUserResponse struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Email    string `json:"email,omitempty"`
	WechatID string `json:"wechatId,omitempty"`
}

type loginResponse struct {
	Token string              `json:"token"`
	User  currentUserResponse `json:"user"`
}

func currentUserFromModel(user models.User) currentUserResponse {
	return currentUserResponse{
		ID:       user.ID,
		Name:     user.Name,
		Username: user.Username,
		Phone:    user.Phone,
		Email:    user.Email,
		WechatID: user.WechatID,
	}
}
