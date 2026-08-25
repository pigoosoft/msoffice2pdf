package domain

// Archive / conversion error codes (stored on upload_history.error_code).
const (
	ErrRetryLimitExceeded  = "ERR_RETRY_LIMIT_EXCEEDED"
	ErrDocPasswordRequired = "ERR_DOC_PASSWORD_REQUIRED"
	ErrDocPasswordWrong    = "ERR_DOC_PASSWORD_WRONG"
)
