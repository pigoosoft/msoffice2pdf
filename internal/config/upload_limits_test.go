package config_test

import (
	"reflect"
	"testing"

	"msoffice2pdf/internal/config"
)

func TestAllowedExtsForClient(t *testing.T) {
	u := config.UploadConfig{
		AllowedExts: []string{"*.DOCX", ".wps", "et", "*", "*.*", "  ", "*.docx"},
	}
	got := u.AllowedExtsForClient()
	want := []string{".docx", ".wps", ".et"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}
