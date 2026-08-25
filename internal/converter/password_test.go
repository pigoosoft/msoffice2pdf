package converter

import (
	"context"
	"errors"
	"testing"
)

func TestIsPasswordError(t *testing.T) {
	if IsPasswordError(errors.New("boom")) {
		t.Fatal("plain error")
	}
	if !IsPasswordError(ErrPasswordRequired) || !IsPasswordError(ErrPasswordWrong) {
		t.Fatal("sentinels")
	}
	if !IsPasswordError(errors.Join(ErrPasswordWrong, errors.New("ole"))) {
		t.Fatal("wrapped")
	}
}

func TestPasswordEnvRoundTrip(t *testing.T) {
	t.Setenv(DocPasswordEnv, "s3cret")
	if got := PasswordFromEnv(); got != "s3cret" {
		t.Fatalf("got %q", got)
	}
}

func TestParseWorkerPasswordError(t *testing.T) {
	if !errors.Is(ParseWorkerPasswordError("converter: ERR_DOC_PASSWORD_REQUIRED"), ErrPasswordRequired) {
		t.Fatal("required")
	}
	if !errors.Is(ParseWorkerPasswordError("ERR_DOC_PASSWORD_WRONG"), ErrPasswordWrong) {
		t.Fatal("wrong")
	}
	if ParseWorkerPasswordError("timeout") != nil {
		t.Fatal("non-password")
	}
}

func TestMapOfficeOpenError(t *testing.T) {
	if !errors.Is(mapOfficeOpenError(errors.New("Password"), "", true), ErrPasswordRequired) {
		t.Fatal("required")
	}
	if !errors.Is(mapOfficeOpenError(errors.New("Password"), "x", true), ErrPasswordWrong) {
		t.Fatal("wrong")
	}
}

func TestMapOpenOfficeConvertError(t *testing.T) {
	srcFailed := errors.New("source file could not be loaded")

	got := mapOpenOfficeConvertError(errors.New("killed"), "", context.DeadlineExceeded)
	if errors.Is(got, ErrPasswordRequired) || errors.Is(got, ErrPasswordWrong) {
		t.Fatalf("timeout mapped to password: %v", got)
	}
	if !errors.Is(got, context.DeadlineExceeded) {
		t.Fatalf("timeout: got %v", got)
	}

	got = mapOpenOfficeConvertError(srcFailed, "", nil)
	if got != ErrPasswordRequired {
		t.Fatalf("empty password: got %v", got)
	}

	got = mapOpenOfficeConvertError(srcFailed, "x", nil)
	if got != ErrPasswordWrong {
		t.Fatalf("non-empty password: got %v", got)
	}
}
