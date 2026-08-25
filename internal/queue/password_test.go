package queue

import (
	"testing"

	"msoffice2pdf/internal/domain"
)

func TestPasswordCacheRoundTrip(t *testing.T) {
	q := &Queue{passwords: map[int64]string{}}
	q.setPassword(7, "secret")
	if q.passwordFor(7) != "secret" {
		t.Fatal("cache")
	}
	q.clearPassword(7)
	if q.passwordFor(7) != "" {
		t.Fatal("cleared")
	}
}

func TestIsPasswordFailCode(t *testing.T) {
	if !isPasswordFailCode(domain.ErrDocPasswordRequired) {
		t.Fatal("required")
	}
	if !isPasswordFailCode(domain.ErrDocPasswordWrong) {
		t.Fatal("wrong")
	}
	if isPasswordFailCode(domain.ErrRetryLimitExceeded) {
		t.Fatal("retry limit is not password")
	}
	if isPasswordFailCode("") || isPasswordFailCode("timeout") {
		t.Fatal("other codes")
	}
}
