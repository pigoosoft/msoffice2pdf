package domain

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	UID       string    `gorm:"size:64;uniqueIndex;not null"`
	PwdHash   string    `gorm:"size:255;not null"`
	Token     string    `gorm:"size:255"`
	Role      int8      `gorm:"not null;default:0"`
	Status    int8      `gorm:"not null;default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (User) TableName() string { return "user" }

type Upload struct {
	ID            int64          `gorm:"primaryKey;autoIncrement"`
	FileID        string         `gorm:"column:fileid;size:64;uniqueIndex;not null"`
	UserID        int64          `gorm:"not null;index"`
	OriginalName  string         `gorm:"size:255;not null"`
	StoredName    string         `gorm:"size:255;not null"`
	FilePath      string         `gorm:"size:512;not null"`
	FileSize      int64          `gorm:"not null"`
	Status        string         `gorm:"size:20;not null"`
	ErrorMsg      string         `gorm:"type:text"`
	RetryCount    int            `gorm:"not null;default:0"`
	LastFailedAt  *time.Time     `gorm:"column:last_failed_at"`
	WatermarkText string         `gorm:"size:255;not null;default:''"`
	RequestID     string         `gorm:"column:request_id;size:128;not null;default:'';index"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

func (Upload) TableName() string { return "upload" }

type Pdf struct {
	ID        int64      `gorm:"primaryKey;autoIncrement"`
	FileID    string     `gorm:"column:fileid;size:64;uniqueIndex;not null"`
	UploadID  int64      `gorm:"not null;index"`
	UserID    int64      `gorm:"not null;index"`
	Filename  string     `gorm:"size:255;not null"`
	FilePath  string     `gorm:"size:512;not null"`
	FileSize  int64      `gorm:"not null"`
	Status    string     `gorm:"size:20;not null"`
	WarnCode  string     `gorm:"size:64;not null;default:''"`
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiredAt *time.Time
}

func (Pdf) TableName() string { return "pdf" }

type PdfLog struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	PdfID     int64     `gorm:"not null;index"`
	FileID    string    `gorm:"column:fileid;size:64;not null;default:'';index"`
	Action    string    `gorm:"size:50;not null"`
	Detail    string    `gorm:"type:text"`
	IPAddress string    `gorm:"column:ip_address;size:45"`
	UserAgent string    `gorm:"column:user_agent;size:255"`
	CreatedAt time.Time
}

func (PdfLog) TableName() string { return "pdflog" }

type ExpiredUpload struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	UploadID     int64     `gorm:"not null;index"`
	FileID       string    `gorm:"column:fileid;size:64;not null"`
	UserID       int64     `gorm:"not null;index"`
	OriginalName string    `gorm:"size:255"`
	MovedPath    string    `gorm:"size:512"`
	ErrorCode    string    `gorm:"size:64;not null;default:''"`
	ErrorMsg     string    `gorm:"type:text"`
	ExpiredAt    time.Time `gorm:"not null"`
	CreatedAt    time.Time
}

func (ExpiredUpload) TableName() string { return "expired_upload" }

// UploadHistory is a terminal snapshot of an upload (completed / failed over-limit / deleted).
type UploadHistory struct {
	ID            int64          `gorm:"primaryKey;autoIncrement"`
	UploadID      int64          `gorm:"not null;index"` // original upload.id; pdf.upload_id still points here
	FileID        string         `gorm:"column:fileid;size:64;index;not null"`
	UserID        int64          `gorm:"not null;index"`
	OriginalName  string         `gorm:"size:255"`
	StoredName    string         `gorm:"size:255"`
	FileSize      int64          `gorm:"not null;default:0"`
	FinalStatus   string         `gorm:"size:20;not null"` // completed | failed | deleted
	ErrorCode     string         `gorm:"size:64;not null;default:''"`
	ErrorMsg      string         `gorm:"type:text"`
	RetryCount    int            `gorm:"not null;default:0"`
	RequestID     string         `gorm:"column:request_id;size:128;not null;default:''"`
	WatermarkText string         `gorm:"size:255;not null;default:''"`
	ArchiveDir    string         `gorm:"size:16;not null"` // expired | trash
	MovedPath     string         `gorm:"size:512"`         // relative under archive root; may be cleared by History TTL
	UploadedAt    time.Time      `gorm:"not null"`
	FinishedAt    time.Time      `gorm:"not null"`
	CreatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

func (UploadHistory) TableName() string { return "upload_history" }

// PressureSample is a periodic snapshot of queue depth and host resources (admin perf overview).
type PressureSample struct {
	ID              int64     `gorm:"primaryKey;autoIncrement"`
	SampledAt       time.Time `gorm:"index;not null"`
	Pending         int64
	Queued          int64
	Converting      int64
	Failed          int64
	ChannelLen      int
	WorkersCur      int
	WorkersMax      int
	WorkersMin      int
	LogBacklogBytes int64
	HeapAlloc       uint64
	RamAvail        uint64
	DiskFreeMin     uint64
	DegradeReason   string `gorm:"size:32;not null;default:''"`
}

func (PressureSample) TableName() string { return "pressure_sample" }
