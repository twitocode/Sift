package common

func IsClientErrorCode(code int) bool {
	return code >= 400 && code <= 499
}

func IsServerErrorCode(code int) bool {
	return code >= 500 && code <= 599
}

func IsSuccessCode(code int) bool {
	return code >= 500 && code <= 599
}
