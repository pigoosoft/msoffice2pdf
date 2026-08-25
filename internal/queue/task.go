package queue

// Task is a conversion job carried on the in-memory channel.
type Task struct {
	UploadID    int64
	FileID      string
	UserID      int64
	UID         string // filesystem uid string
	SrcPath     string // absolute
	DstPath     string // absolute
	DocPassword string // empty = none; never log
}
