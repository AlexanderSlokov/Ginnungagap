package pypi

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	typeJSON = "application/vnd.pypi.simple.v1+json"
	typeHTML = "application/vnd.pypi.simple.v1+html"
)

// contentType selects the response format from the Accept header.
//
// shortcut: look for the media type names and ignore the q-values.
// pip and uv put the JSON type first when they want it, and a browser does not send it.
// Limit: a client that sends JSON with q=0 gets JSON. Upgrade: parse q-values if a client needs it.
func contentType(r *http.Request) string {
	accept := r.Header.Get("Accept")
	switch {
	case strings.Contains(accept, typeJSON):
		return typeJSON
	case strings.Contains(accept, typeHTML):
		return typeHTML
	}
	return "text/html; charset=utf-8"
}

var rootHTML = template.Must(template.New("root").Parse(`<!DOCTYPE html>
<html><head><meta name="pypi:repository-version" content="1.0"><title>Simple index</title></head><body>
{{range .}}<a href="{{.}}/">{{.}}</a><br>
{{end}}</body></html>
`))

func renderRoot(w http.ResponseWriter, r *http.Request, names []string) {
	ct := contentType(r)
	w.Header().Set("Content-Type", ct)
	if ct == typeJSON {
		type project struct {
			Name string `json:"name"`
		}
		doc := struct {
			Meta     map[string]string `json:"meta"`
			Projects []project         `json:"projects"`
		}{Meta: map[string]string{"api-version": "1.0"}, Projects: []project{}}
		for _, n := range names {
			doc.Projects = append(doc.Projects, project{n})
		}
		json.NewEncoder(w).Encode(doc)
		return
	}
	rootHTML.Execute(w, names)
}

type linkView struct {
	Filename, URL, SHA256, RequiresPython, YankedReason, Metadata string
	Yanked                                                        bool
}

var projectHTML = template.Must(template.New("project").Parse(`<!DOCTYPE html>
<html><head><meta name="pypi:repository-version" content="1.0"><title>Links for {{.Name}}</title></head><body>
<h1>Links for {{.Name}}</h1>
{{range .Links}}<a href="{{.URL}}#sha256={{.SHA256}}"{{with .RequiresPython}} data-requires-python="{{.}}"{{end}}{{if .Yanked}} data-yanked="{{.YankedReason}}"{{end}}{{with .Metadata}} data-core-metadata="sha256={{.}}" data-dist-info-metadata="sha256={{.}}"{{end}}>{{.Filename}}</a><br>
{{end}}</body></html>
`))

func fileURL(repo string, f File) string {
	return "/pypi/" + url.PathEscape(repo) + "/files/" + strconv.FormatInt(f.ID, 10) + "/" + url.PathEscape(f.Filename)
}

func renderProject(w http.ResponseWriter, r *http.Request, repo, name string, files []File) {
	ct := contentType(r)
	w.Header().Set("Content-Type", ct)
	if ct != typeJSON {
		var links []linkView
		for _, f := range files {
			l := linkView{Filename: f.Filename, URL: fileURL(repo, f), SHA256: f.SHA256}
			if f.RequiresPython != nil {
				l.RequiresPython = *f.RequiresPython
			}
			if f.Yanked != nil {
				l.Yanked, l.YankedReason = true, *f.Yanked
			}
			if f.MetadataSHA256 != nil {
				l.Metadata = *f.MetadataSHA256
			}
			links = append(links, l)
		}
		projectHTML.Execute(w, struct {
			Name  string
			Links []linkView
		}{name, links})
		return
	}

	type jsonFile struct {
		Filename       string            `json:"filename"`
		URL            string            `json:"url"`
		Hashes         map[string]string `json:"hashes"`
		RequiresPython *string           `json:"requires-python,omitempty"`
		Yanked         any               `json:"yanked"`
		Size           *int64            `json:"size,omitempty"`
		UploadTime     string            `json:"upload-time,omitempty"`
		CoreMetadata   any               `json:"core-metadata"`
		DistInfo       any               `json:"data-dist-info-metadata"`
	}
	doc := struct {
		Meta     map[string]string `json:"meta"`
		Name     string            `json:"name"`
		Files    []jsonFile        `json:"files"`
		Versions []string          `json:"versions,omitempty"`
	}{Meta: map[string]string{"api-version": "1.0"}, Name: name, Files: []jsonFile{}}

	allSized := true
	seen := map[string]bool{}
	var versions []string
	for _, f := range files {
		jf := jsonFile{Filename: f.Filename, URL: fileURL(repo, f), Hashes: map[string]string{"sha256": f.SHA256},
			RequiresPython: f.RequiresPython, Yanked: false, Size: f.Size, CoreMetadata: false, DistInfo: false}
		if f.Yanked != nil {
			jf.Yanked = true
			if *f.Yanked != "" {
				jf.Yanked = *f.Yanked
			}
		}
		if f.UploadTime != nil {
			jf.UploadTime = f.UploadTime.UTC().Format(time.RFC3339Nano)
		}
		if f.MetadataSHA256 != nil {
			h := map[string]string{"sha256": *f.MetadataSHA256}
			jf.CoreMetadata, jf.DistInfo = h, h
		}
		allSized = allSized && f.Size != nil
		if !seen[f.Version] {
			seen[f.Version] = true
			versions = append(versions, f.Version)
		}
		doc.Files = append(doc.Files, jf)
	}
	// API version 1.1 (PEP 700) requires "size" for every file and the "versions" list.
	if allSized {
		doc.Meta["api-version"] = "1.1"
		doc.Versions = versions
	}
	json.NewEncoder(w).Encode(doc)
}
