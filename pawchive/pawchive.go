package pawchive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

type Creator struct {
    ID      string `json:"id"`
    Name    string `json:"name"`
    Service string `json:"service"`
    Indexed string `json:"indexed"`
    Updated string `json:"updated"`
}

type Attachment struct {
    Name string `json:"name"`
    Path string `json:"path"`
		Server string `json:"server"`
}

type File struct {
    Name string `json:"name"`
    Path string `json:"path"`
		Server string `json:"server"`
}

type Post struct {
    ID          string       `json:"id"`
    User        string       `json:"user"`
    Service     string       `json:"service"`
    Title       string       `json:"title"`
    Content     string       `json:"content"`
    Embed       map[string]any `json:"embed,omitempty"`
    Shared      string       `json:"shared"`
    Added       string       `json:"added"`
    Edited      string       `json:"edited"`
    Published   string       `json:"published"`
    File        File         `json:"file"`
    Attachments []Attachment `json:"attachments"`
}

type CreatorResponse struct {
    Creator Creator `json:"creator"`
    Posts   []Post  `json:"posts"`
}

//Returns the post's main file followed by every attachment.
func (p Post) AllFiles() []File {
	seen := map[string]bool{}
	var files []File
	if p.File != (File{}) {
	  addFile(p.File.Name, fileUrl(p.File.Name, p.File.Path, p.File.Server), &files, seen)
	}
	for _, attach := range p.Attachments {
		addFile(attach.Name, fileUrl(attach.Name, attach.Path, attach.Server), &files, seen)
	}
	return files
}

func fileUrl(name, path, server string) string {
	return fmt.Sprintf("%s/data/%s?f=%s", server, path, strings.ReplaceAll(name, " ", "%20"))
}

func addFile(name, filepath string, files *[]File, seen map[string]bool) {
	if filepath == "" || seen[filepath] {
		return 
	}
	seen[filepath] = true
	if name == "" {
		name = path.Base(filepath)
	}

	*files = append(*files, File{Name: name, Path: filepath})
	
}

//Returns a hash of every field that matters when deciding whether a post
// has been edited since we last saved it.
func (p Post) Fingerprint() string {
	post := struct{
		Title string
		Content string
		Edited string
		File File
		Attachments []Attachment
		Embed map[string]any
	}{p.Title, p.Content, p.Edited, p.File, p.Attachments, p.Embed}

	data, _ := json.Marshal(post)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
