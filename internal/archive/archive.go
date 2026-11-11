package archive

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/KamogeloMahlake/pawchive_downloader/internal/clients"
	"github.com/KamogeloMahlake/pawchive_downloader/internal/storage"
	"github.com/KamogeloMahlake/pawchive_downloader/pawchive"
)

// Syncer holds everything RUn needs.
type Syncer struct {
	Client *clients.Client
	Store *storage.Store
	DataUrl string // where files are served from
	Workers int // parallel file downloads per post
	Skipfiles bool
	Log func(format string, args ...any)
}

func (s *Syncer) logf(format string, args ...any) {
	if s.Log != nil {
		s.Log(format, args...)
	}
}

func (s *Syncer) fileUrl(p string) (string, error) {
	if strings.HasPrefix(p, "https://") || strings.HasPrefix(p, "http://") {
		return p, nil
	}
	u, err := url.Parse(s.DataUrl)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("Invalid data url %s", s.DataUrl)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(p, "/")
	return u.String(), nil
}

// localName gives a file a stable, collision-free name inside files/
func localName(f pawchive.File) string {
	sum := sha1.Sum([]byte(f.Path))
	return hex.EncodeToString(sum[:4]) + "_" + storage.SafeName(f.Name, 120)
}

//mergeFiles builds the file list for a post.
func mergeFiles(post pawchive.Post, old []storage.FileRecord) []storage.FileRecord {
	prev := map[string]storage.FileRecord{}
	for _, record := range old {
		prev[record.RemotePath] = record
	}
	var output []storage.FileRecord 
	for _, file := range post.AllFiles() {
		if record, ok := prev[file.Path]; ok {
			record.Name = file.Name
			output = append(output, record)
			continue
		}
		output = append(output, storage.FileRecord{Name: file.Name, RemotePath: file.Path, LocalName: localName(file)})
	}
	return output
}

func (s *Syncer) downloadOne(ctx context.Context, dir string, file *storage.FileRecord) error {
	u, err := s.fileUrl(file.RemotePath)
	if err != nil {
		return err
	}
	size, err := s.Client.DownloadFile(ctx, u, filepath.Join(dir, file.LocalName))
	if err != nil {
		return err
	}
	file.Size = size
	return nil
}

// downloadFiles fetches every file of the post that is not yet done.
func (s *Syncer) downloadFiles(ctx context.Context, index *storage.Index, record *storage.PostRecord, report *Report) error {
	dir, err := s.Store.FilesDir(index.Service, index.UserID, record.ID)
	if err != nil {
		return err
	}
	var pending []*storage.FileRecord
	for i := range record.Files {
		file := &record.Files[i]
		if file.Done {
			if st, err := os.Stat(filepath.Join(dir, file.LocalName)); err == nil && st.Size() == file.Size {
				continue
			}
			file.Done = false
		}
		pending = append(pending, file)
	}
	if len(pending) == 0 {
		return nil
	}

	jobs := make(chan *storage.FileRecord)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for w := 0; w < s.Workers; w++ {
		wg.Add(1)
		a := func() {
			defer wg.Done()
			for f := range jobs {
				err := s.downloadOne(ctx, dir, f)
				mu.Lock()
				if err != nil {
					f.Done, f.Error = false, err.Error()
					report.FilesFailed++
					s.logf("   failed    %s: %v", f.Name, err)
				} else {
					f.Done, f.Error = true, ""
					report.FilesOK++
					s.logf("  saved     %s (%d bytes)", f.Name, f.Size)
				}
				mu.Unlock()
			}
		}
		wg.Go(a)
	}

	for _, f := range pending {
		if ctx.Err() != nil {
			break
		}
		jobs <- f
	}
	close(jobs)
	wg.Wait()
	return ctx.Err()
}


// syncPost handles one post: classify it, save it, download its files.
func (s *Syncer) syncPost(ctx context.Context, index *storage.Index, post pawchive.Post, report *Report) error {
	fingerprint := post.Fingerprint()
	record, exists := index.Posts[post.ID]
	now := time.Now().UTC()

	switch {
	case !exists:
		record = &storage.PostRecord{ID: post.ID, FirstSeen: now}
		index.Posts[post.ID] = record
		report.New++
		s.logf("New       %s  %s", post.ID, post.Title)
	case record.FingerPrint != fingerprint:
		report.Updated++
		s.logf("Updated    %s  %s", post.ID, post.Title)
	default:
		report.Unchanged++
	}

	if !exists || record.FingerPrint != fingerprint {
		if err := s.Store.SavePost(index.Service, index.UserID, post); err != nil {
			return err
		}
		record.LastChanged = now
	}

	record.Title, record.Published, record.Edited, record.FingerPrint = post.Title, post.Published, post.Edited, record.FingerPrint
	record.Files = mergeFiles(post, record.Files)

	if !s.Skipfiles {
		if err := s.downloadFiles(ctx, index, record, report); err != nil {
			return err
		}
	}
	return s.Store.SaveIndex(index)
}

// Run synchronises one creator.
func (s *Syncer) Run(ctx context.Context, service, userId string) (Report, error) {
	var report Report
	if s.Workers < 1 {
		s.Workers = 1
	}

	if _, err := s.Store.CreatorDir(service, userId); err != nil {
		return report, err
	}

	creator, err := s.Client.GetCreator(ctx, service, userId)
	if err != nil {
		return report, fmt.Errorf("Fetching creator: %w", err)
	}
	creator.Service, creator.ID = service, userId
	if err := s.Store.SaveCreator(creator); err != nil {
		return report, err
	}

	index, err := s.Store.OpenIndex(service, userId)
	if err != nil {
		return report, err
	}
	index.Name = creator.Name

	s.logf("Fetching post list fot %s/%s (%s)...", service, userId, creator.Name)
	posts, err := s.Client.GetPosts(ctx, service, userId)
	if err != nil {
		return report, fmt.Errorf("Fetching posts: %w", err)
	}

	s.logf("Server list %d posts", len(posts))

	remote := map[string]bool{}
	for _, post := range posts {
		remote[post.ID] = true
		if err := ctx.Err(); err != nil {
			return report, err
		}

		if err := s.syncPost(ctx, index, post, &report); err != nil {
			_ = s.Store.SaveIndex(index)
			return report, err
		} 
	}

	for id, record := range index.Posts {
		if !remote[id] && !record.RemoteMissing {
			record.RemoteMissing = true
			report.MissingRemote++
			s.logf("Post %s (%q) is no longer listed by the server; keeping local copy", id, record.Title)
		} else if remote[id] {
			record.RemoteMissing = false
		}
	}
	index.LastSync = time.Now().UTC()
	return report, s.Store.SaveIndex(index)
}
