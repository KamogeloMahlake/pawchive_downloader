package viewer

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/KamogeloMahlake/pawchive_downloader/internal/storage"
)


type server struct{ st *storage.Store }

func render(w http.ResponseWriter, t *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; media-src 'self'; style-src 'unsafe-inline'; form-action 'self'")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (s *server) home(w http.ResponseWriter, r *http.Request) {
	list, err := s.st.ListCreators()
	if err != nil {
		http.Error(w, "cannot read data folder", http.StatusInternalServerError)
		return
	}

	type row struct {
		*storage.Index
		Sync string
	}
	var rows []row
	for _, index := range list {
		sync := "never"
		if !index.LastSync.IsZero() {
			sync = index.LastSync.Local().Format("2006-01-02 15:04")
		}
		rows = append(rows, row{index, sync})
	}
	render(w, tmplHome, map[string]any{"Title": "Creators", "Creators": rows})
}

func (s *server) creator(w http.ResponseWriter, r *http.Request) {
	service, id := r.PathValue("service"), r.PathValue("id")
	index, err := s.st.LoadIndex(service, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	type row struct {
		*storage.PostRecord
		Day string
	}

	var rows []row
	for _, post := range index.SortedPost() {
		if query != "" && !strings.Contains(strings.ToLower(post.Title), query) {
			continue 
		}
		rows = append(rows, row{post, day(post.Published)})
	}
	render(w, tmplCreator, map[string]any{
		"Title": index.Name, "Name": index.Name, "Service": index.Service, "UserId": index.UserID,
		"Count": len(index.Posts), "Query": query, "Posts":rows,
	})
}

func (s *server) post(w http.ResponseWriter, r *http.Request) {
	service, id, postId := r.PathValue("service"), r.PathValue("id"), r.PathValue("post")
	index, err := s.st.LoadIndex(service, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	post, err := s.st.LoadPost(service, id, postId)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	type fileView struct {
		Name, Kind, Url, Error string
		Size int64
		Missing bool
	}
	var files []fileView
	if record := index.Posts[postId]; record != nil {
		for _, file := range record.Files {
			files = append(files, fileView{
				Name: file.Name, Kind: FileKind(file.Name), Size: file.Size, Error: file.Error, Missing: !file.Done,
				Url: "/files/" + service + "/" + id + "/" + postId + "/" + file.LocalName,
			})
		}
	}
	render(w, tmplPost, map[string]any{
		"Title": post.Title, "Service": service, "UserID": id, "Creator": index.Name,
		"Post": post, "Text": StripHTML(post.Content), "Files": files,
	})
}

func (s *server) file( w http.ResponseWriter, r *http.Request) {
	service, id, postId, name := r.PathValue("service"), r.PathValue("id"), r.PathValue("post"), r.PathValue("name")
	dir, err := s.st.FilesDir(service, id, postId)
	if err != nil || storage.ValidSegment(name) != nil {
		http.NotFound(w, r)
		return
	}
	if FileKind(name) == "other" {
		w.Header().Set("Content-Disposition", "attachment")
	}
	http.ServeFile(w, r, filepath.Join(dir, name))
}

func Serve(ctx context.Context, st *storage.Store, addr string, logw io.Writer) error {
	s := &server{st: st}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /c/{service}/{id}", s.creator)
	mux.HandleFunc("GET /p/{service}/{id}/{post}", s.post)
	mux.HandleFunc("GET /files/{service}/{id}/{post}/{name}", s.file)

	srv := &http.Server{
		Addr: addr,
		Handler: secure(mux),
		ReadHeaderTimeout: time.Second * 10,
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	fmt.Fprintf(logw, "viewer running at http://%s (Ctrl+C to stop)\n", addr)
	select {
	case err := <- errc:
		return err
	case <-ctx.Done():
		shut, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()
		return srv.Shutdown(shut)
	}
}
