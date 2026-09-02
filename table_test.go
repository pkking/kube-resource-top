package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestTableLineAlignsANSICellsByDisplayWidth(t *testing.T) {
	plain := tableLine("", []int{8}, "N/A", "tail")
	coloured := tableLine("", []int{8}, gray.Render("N/A"), "tail")
	if strings.Index(plain, "|") != strings.Index(coloured, "|") {
		t.Fatalf("separators differ: %q vs %q", plain, coloured)
	}
	if lipgloss.Width(plain) != lipgloss.Width(coloured) {
		t.Fatalf("display widths differ")
	}
}
