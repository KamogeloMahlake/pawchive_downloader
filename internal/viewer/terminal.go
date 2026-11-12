package viewer

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/KamogeloMahlake/pawchive_downloader/internal/storage"
)

// PrintCreators lists every downloaded creator
func PrintCreators(w io.Writer, store *storage.Store) error {
	list, err := store.ListCreators()
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Fprintln(w, "Nothing downloaded yet. Try: pawchive download <service> <id>")
		return nil
	}
	fmt.Fprintf(w, "%-10s %-14s %-28s %6s  %s\n", "SERVICE", "ID", "NAME", "POSTS", "LAST SYNC")
	for _, index := range list {
		sync := "never"
		if !index.LastSync.IsZero() {
			sync = index.LastSync.Local().Format("2006-01-02 15:04")
		}
		fmt.Fprintf(w, "%-10s %-14s %-28s %6d  %s\n", index.Service, index.UserID, short(index.Name, 28), len(index.Posts), sync)
	}
	return nil
}

 
// PrintPosts lists the posts of one creator, newest first.
func PrintPosts(w io.Writer, st *storage.Store, service, id string) error {
	ix, err := st.LoadIndex(service, id)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "%s (%s/%s) - %d posts\n\n", ix.Name, ix.Service, ix.UserID, len(ix.Posts))
	fmt.Fprintf(w, "%-12s %-12s %5s  %s\n", "POST", "PUBLISHED", "FILES", "TITLE")
	for _, p := range ix.SortedPost() {
		mark := ""
		if p.RemoteMissing {
			mark = "  [removed on server]"
		}
		fmt.Fprintf(w, "%-12s %-12s %5d  %s%s\n", p.ID, day(p.Published), len(p.Files), short(p.Title, 60), mark)
	}
	return nil
}

// PrintPost shows one post: header, text, and the files with their local paths.
func PrintPost(w io.Writer, st *storage.Store, service, id, postID string) error {
	post, err := st.LoadPost(service, id, postID)
	if err != nil {
		return err
	}
	ix, err := st.LoadIndex(service, id)
	if err != nil {
		return err
	}
	dir, err := st.FilesDir(service, id, postID)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "%s\n%s\n", post.Title, strings.Repeat("=", len([]rune(post.Title))))
	fmt.Fprintf(w, "post %s by %s (%s)\n", post.ID, ix.Name, service)
	fmt.Fprintf(w, "published: %s   edited: %s\n", post.Published, post.Edited)
	if len(post.Tags) > 0 {
		fmt.Fprintf(w, "tags: %s\n", strings.Join(post.Tags, ", "))
	}
	fmt.Fprintf(w, "\n%s\n\n", StripHTML(post.Content))

	rec := ix.Posts[postID]
	if rec == nil || len(rec.Files) == 0 {
		fmt.Fprintln(w, "(no files)")
		return nil
	}
	fmt.Fprintln(w, "Files:")
	for _, f := range rec.Files {
		state := "ok"
		if !f.Done {
			state = "NOT DOWNLOADED"
			if f.Error != "" {
				state += ": " + f.Error
			}
		}
		fmt.Fprintf(w, "  %-40s %10d bytes  %s\n    %s\n", short(f.Name, 40), f.Size, state, filepath.Join(dir, f.LocalName))
	}
	return nil
}
