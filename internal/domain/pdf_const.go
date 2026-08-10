package domain

const (
	PdfStatusGenerating = "generating"
	PdfStatusCompleted  = "completed"
	PdfStatusFailed     = "failed"
	PdfStatusExpired    = "expired"
	PdfStatusDeleted    = "deleted"

	PdfLogActionGenerate = "generate"
	PdfLogActionDownload = "download"
	PdfLogActionDelete   = "delete"
	PdfLogActionExpire   = "expire"

	PdfWarnWatermark = "WARN_WATERMARK"
)
