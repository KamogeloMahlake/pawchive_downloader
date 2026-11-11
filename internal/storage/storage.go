package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/KamogeloMahlake/pawchive_downloader/pawchive"
)

//Store is a handle on the data directory
type Store struct {
	Root string
}

var ErrorNotStored = errors.New("Not downloaded yet")

func NewStore(dir string) *Store {
	return &Store{Root: dir}
}

//ValidSegment check that s is safe to use as one folder or filename.
func ValidSegment(s string) error {
	switch  {
	case s == "":
		return errors.New("Empty Name")
	case strings.HasPrefix(s, "."):
		return fmt.Errorf("Invalid name %s: must not start with a dot", s)
	case len(s) > 200:
		return errors.New("Name too long")
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`/\:*?"<>|`, r) {
			return fmt.Errorf("Invalid character %q in name %s", r, s)
		}
	}
	return nil
}

//SafeName turns string into something that passes ValidSegment.
func SafeName(s string, max int) string {
	var builder strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '.', r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	name := strings.Trim(builder.String(), "._")
	if name == "" {
		return "file"
	}
	if max > 0 && len(name) > max {
		ext := filepath.Ext(name)
		if len(ext) > 10 {
			ext = ""
		}
		base := strings.TrimSuffix(name, ext)

		for len(base)+len(ext) > max && base != "" {
			_, size := utf8.DecodeLastRuneInString(base)
			base = base[:len(base)-size]
		}
		base = strings.TrimRight(base, "._")
		if base == "" {
			return "file"
		}
		name = base + ext
	}
	return name
}

//CreatorDir returns <root>/<service>/<id> after validating both parts. 
func (s *Store) CreatorDir(service, id string) (string, error) {
	if err := ValidSegment(service); err != nil {
		return "", fmt.Errorf("service: %w", err)
	}
	if err := ValidSegment(id); err != nil {
		return "", fmt.Errorf("Creater id: %w", err)
	}
	return filepath.Join(s.Root, service, id), nil
}

//PostDir returns the folder of the on post 
func (s *Store) PostDir(service, id, postId string) (string, error) {
	dir, err := s.CreatorDir(service, id)
	if err != nil {
		return "", err
	}
	if err := ValidSegment(postId); err != nil {
		return "", fmt.Errorf("PostId: %w", err)
	}
	return filepath.Join(dir, "posts", postId), nil
}

//FilesDir returns the folder holding a post's downloaded files.
func (s *Store) FilesDir(service, id, postId string) (string, error) {
	dir, err := s.PostDir(service, id, postId)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "files"), nil
}

//writFileAtomic writes data to a temporary file and renames it over path, so a crash can never leave a half-written JSON file behind
func writFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}

	tempName := temp.Name()
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(tempName)
		return err
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempName)
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		os.Remove(tempName)
		return err
	}
	return nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writFileAtomic(path, data)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

//LoadIndex read a creator's index.
func (s *Store) LoadIndex(service, id string) (*Index, error) {
	dir, err := s.CreatorDir(service, id)
	if err != nil {
		return nil, err
	}

	var index Index
	if err := readJSON(filepath.Join(dir, "index.json"), &index); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s/%s: %w", service, id, ErrorNotStored)
		}
		return nil, err
	}
	if index.Posts == nil {
		index.Posts = map[string]*PostRecord{}
	}
	return &index, nil
}

//OpenIndex returns an empty index for a new creator.
func (s *Store) OpenIndex(service, id string) (*Index, error) {
	index, err := s.LoadIndex(service, id)
	if errors.Is(err, ErrorNotStored) {
		return &Index{
			Service: service,
			UserID: id,
			Posts: map[string]*PostRecord{},
		}, nil
	}
	return index, err
}

//SaveIndex writes the index to disk
func (s *Store) SaveIndex(index *Index) error {
	dir, err := s.CreatorDir(index.Service, index.UserID)
	if err != nil {
		return err
	}
	return writeJSON(filepath.Join(dir, "index.json"), index)
}

// SaveCreator stores the profile
func (s *Store) SaveCreator(c pawchive.Creator) error {
	dir, err := s.CreatorDir(c.Service, c.ID)
	if err != nil {
		return err
	}
	return writeJSON(filepath.Join(dir, "creator.json"), c)
}

//SavePost writes post.json. If a different is already stored, the old is move to history/ first.
func (s *Store) SavePost(service, id string, p pawchive.Post) error {
	dir, err := s.PostDir(service, id, p.ID)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "post.json")
	if old, err := os.ReadFile(path); err == nil {
		var prev pawchive.Post
		if json.Unmarshal(old, &prev) == nil && prev.Fingerprint() != p.Fingerprint() {
			name := time.Now().UTC().Format("20060102T150405.000000000Z") + ".json"
			if err := writFileAtomic(filepath.Join(dir, "history", name), old); err != nil {
				return nil
			}
		} 
	}
	return writeJSON(path, p)
}

// LoadPost reads the stored post.json.
func (s *Store) LoadPost(service, id, postId string) (pawchive.Post, error) {
	var post pawchive.Post
	dir, err := s.PostDir(service, id, postId)
	if err != nil {
		return post, err
	}
	if err := readJSON(filepath.Join(dir, "post.json"), &post); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return post, fmt.Errorf("Post %s: %w", postId, ErrorNotStored)
		}
		return post, err
	}
	return post, nil
}

//ListCreators return the index of every creator in the data folder.
func (s *Store) ListCreators() ([]*Index, error) {
	var output []*Index
	services, err := os.ReadDir(s.Root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return output, err
	}

	for _, service := range services {
		if !service.IsDir() || ValidSegment(service.Name()) != nil {
			continue
		}
		creators, err := os.ReadDir(filepath.Join(s.Root, service.Name()))
		if err != nil {
			continue
		}

		for _, creator := range creators {
			if !creator.IsDir() || ValidSegment(creator.Name()) != nil {
				continue
			}
			index, err := s.LoadIndex(service.Name(), creator.Name())
			if err != nil {
				continue
			}
			output = append(output, index)
		}
	}

	sort.Slice(output, func(i, j int) bool {
		if output[i].Service != output[j].Service {
			return output[i].Service < output[j].Service
		}
		return output[i].UserID < output[j].UserID
	})
	return output, nil
}
