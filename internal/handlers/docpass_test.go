package handlers

import "testing"

func TestPickDocPassword(t *testing.T) {
	if pickDocPassword("H", "F") != "H" {
		t.Fatal("header wins")
	}
	if pickDocPassword("", "F") != "F" {
		t.Fatal("form")
	}
	if pickDocPassword("  ", "F") != "F" {
		t.Fatal("blank header")
	}
	if pickDocPassword("", "  ") != "" {
		t.Fatal("blank form")
	}
}
