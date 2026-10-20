package pawchive

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
}

type File struct {
    Name string `json:"name"`
    Path string `json:"path"`
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
    Tags        []string     `json:"tags"`
}

type CreatorResponse struct {
    Creator Creator `json:"creator"`
    Posts   []Post  `json:"posts"`
}
