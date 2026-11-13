package storage

// FileRecord tracks one download file of a post.
type FileRecord struct {
	Name string `json:"name"`
	RemotePath string `json:"remote_path"`
	LocalName string `json:"local_name"`
	Server string `json:"server"`
	Size int64 `json:"size"`
	Done bool `json:"done"`
	Error string `json:"error,omitempty"`
}

