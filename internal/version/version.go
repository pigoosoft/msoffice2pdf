// Package version holds build identity for CLI and diagnostics.
package version

// Version is the application release version. Override at link time, e.g.:
//
//	go build -ldflags "-X msoffice2pdf/internal/version.Version=1.2.3" ...
var Version = "0.1.0"

// Copyright is the short copyright line shown by the version command and About dialog.
const Copyright = "Copyright (c) 2026 pigoosoft (pigoosoft@gmail.com)"

// AppName is the product name used in UI and CLI banners.
const AppName = "MSOffice2Pdf"

// Description is a one-line summary of what the product does.
const Description = "HTTP service that converts Microsoft Office documents (Word / Excel / PowerPoint) to PDF via Office COM (Windows) or OpenOffice/LibreOffice, preserving layout as much as possible."
