package viewer

import "html/template"

const pageHead = `<!doctype html><html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}} - Pawchive viewer</title>
<style>
body{font-family:system-ui,sans-serif;max-width:60rem;margin:0 auto;padding:1rem;line-height:1.5;color:#222;background:#fafafa}
a{color:#0b5cad}table{border-collapse:collapse;width:100%}td,th{padding:.35rem .5rem;border-bottom:1px solid #ddd;text-align:left}
.muted{color:#777;font-size:.9rem}.post{white-space:pre-wrap;background:#fff;border:1px solid #ddd;padding:1rem;border-radius:6px}
img,video{max-width:100%;height:auto;display:block;margin:1rem 0;border-radius:4px}.warn{color:#a40000}
input[type=search]{padding:.4rem;width:16rem}
@media (prefers-color-scheme:dark){body{background:#181818;color:#ddd}a{color:#7ab7ff}.post{background:#222;border-color:#444}td,th{border-color:#444}}
</style></head><body><p><a href="/">Home</a></p>`

var tmplHome = template.Must(template.New("home").Parse(pageHead + `
<h1>Downloaded creators</h1>
{{if .Creators}}<table><tr><th>Name</th><th>Service</th><th>Posts</th><th>Last sync</th></tr>
{{range .Creators}}<tr><td><a href="/c/{{.Service}}/{{.UserID}}">{{.Name}}</a></td><td>{{.Service}}</td><td>{{len .Posts}}</td><td class="muted">{{.Sync}}</td></tr>{{end}}</table>
{{else}}<p>Nothing downloaded yet.</p>{{end}}</body></html>`))

var tmplCreator = template.Must(template.New("creator").Parse(pageHead + `
<h1>{{.Name}}</h1><p class="muted">{{.Service}} / {{.UserID}} - {{.Count}} posts</p>
<form method="get"><input type="search" name="q" value="{{.Query}}" placeholder="search titles"> <button>Search</button></form>
<table><tr><th>Published</th><th>Title</th><th>Files</th></tr>
{{range .Posts}}<tr><td class="muted">{{.Day}}</td><td><a href="/p/{{$.Service}}/{{$.UserID}}/{{.ID}}">{{.Title}}</a>{{if .RemoteMissing}} <span class="warn">[removed on server]</span>{{end}}</td><td>{{len .Files}}</td></tr>{{end}}
</table></body></html>`))

var tmplPost = template.Must(template.New("post").Parse(pageHead + `
<p><a href="/c/{{.Service}}/{{.UserID}}">&larr; {{.Creator}}</a></p>
<h1>{{.Post.Title}}</h1>
<p class="muted">published {{.Post.Published}}{{if .Post.Edited}} - edited {{.Post.Edited}}{{end}}{{if .Post.Tags}} - tags: {{range .Post.Tags}}{{.}} {{end}}{{end}}</p>
<div class="post">{{.Text}}</div>
{{range .Files}}
  {{if .Missing}}<p class="warn">{{.Name}}: not downloaded{{if .Error}} ({{.Error}}){{end}}</p>
  {{else if eq .Kind "image"}}<a href="{{.URL}}"><img src="{{.URL}}" alt="{{.Name}}" loading="lazy"></a>
  {{else if eq .Kind "video"}}<video controls preload="metadata" src="{{.URL}}"></video>
  {{else if eq .Kind "audio"}}<p>{{.Name}}</p><audio controls src="{{.URL}}"></audio>
  {{else}}<p><a href="{{.URL}}" download>{{.Name}}</a> <span class="muted">({{.Size}} bytes)</span></p>{{end}}
{{end}}</body></html>`))

