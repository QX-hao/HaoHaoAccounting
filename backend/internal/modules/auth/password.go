package auth

import "golang.org/x/crypto/bcrypt"

// dummyPasswordHash 用于用户名不存在时仍执行一次 bcrypt 校验，降低登录接口的用户枚举时序差异。
const dummyPasswordHash = "$2a$10$6Uo2vGF.9J34Gz6wPXJzO.ZeTqcXN5LK.lXwpHjbtkAn5PLcIboNq"

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func verifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func verifyPasswordOrDummy(hash, password string) bool {
	if hash == "" {
		hash = dummyPasswordHash
	}
	return verifyPassword(hash, password)
}
