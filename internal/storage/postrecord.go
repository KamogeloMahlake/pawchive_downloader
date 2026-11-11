package storage

import "time"

//PostRecord is what the index remembers about the post.
type PostRecord struct {
	ID string `json:"id"`
	Title string `json:"title"`
	Published string `json:"published"`
	Edited string `json:"edited"`
	FingerPrint string `json:"finger_print"`
	Files []FileRecord `json:"files"`
	FirstSeen time.Time `json:"first_seen"`
	LastChanged time.Time `json:"last_changed"`
	RemoteMissing bool `json:"remote_missing"`
}

