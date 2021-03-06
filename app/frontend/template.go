// Copyright 2011 Gary Burd
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

// +build appengine

package app

import (
	"appengine"
	"bytes"
	"errors"
	"fmt"
	"github.com/garyburd/gopkgdoc/doc"
	godoc "go/doc"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"regexp"
	"strings"
	"text/template"
	"time"
)

func mapFmt(kvs ...interface{}) (map[string]interface{}, error) {
	if len(kvs)%2 != 0 {
		return nil, errors.New("map requires even number of arguments.")
	}
	m := make(map[string]interface{})
	for i := 0; i < len(kvs); i += 2 {
		s, ok := kvs[i].(string)
		if !ok {
			return nil, errors.New("even args to map must be strings.")
		}
		m[s] = kvs[i+1]
	}
	return m, nil
}

// relativePathFmt formats an import path as HTML.
func relativePathFmt(importPath string, parentPath interface{}) string {
	if p, ok := parentPath.(string); ok && p != "" && strings.HasPrefix(importPath, p) {
		importPath = importPath[len(p)+1:]
	}
	return urlFmt(importPath)
}

// importPathFmt formats an import with zero width space characters to allow for breeaks.
func importPathFmt(importPath string) string {
	importPath = urlFmt(importPath)
	if len(importPath) > 45 {
		// Allow long import paths to break following "/"
		importPath = strings.Replace(importPath, "/", "/&#8203;", -1)
	}
	return importPath
}

// relativeTime formats the time t in nanoseconds as a human readable relative
// time.
func relativeTime(t time.Time) string {
	const day = 24 * time.Hour
	d := time.Now().Sub(t)
	switch {
	case d < time.Second:
		return "just now"
	case d < 2*time.Second:
		return "one second ago"
	case d < time.Minute:
		return fmt.Sprintf("%d seconds ago", d/time.Second)
	case d < 2*time.Minute:
		return "one minute ago"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", d/time.Minute)
	case d < 2*time.Hour:
		return "one hour ago"
	case d < day:
		return fmt.Sprintf("%d hours ago", d/time.Hour)
	case d < 2*day:
		return "one day ago"
	}
	return fmt.Sprintf("%d days ago", d/day)
}

var (
	h3Open     = []byte("<h3 ")
	h4Open     = []byte("<h4 ")
	h3Close    = []byte("</h3>")
	h4Close    = []byte("</h4>")
	rfcRE      = regexp.MustCompile(`RFC\s+(\d{3,4})`)
	rfcReplace = []byte(`<a href="http://tools.ietf.org/html/rfc$1">$0</a>`)
)

// commentFmt formats a source code comment as HTML.
func commentFmt(v string) string {
	var buf bytes.Buffer
	godoc.ToHTML(&buf, v, nil)
	p := buf.Bytes()
	p = bytes.Replace(p, h3Open, h4Open, -1)
	p = bytes.Replace(p, h3Close, h4Close, -1)
	p = rfcRE.ReplaceAll(p, rfcReplace)
	return string(p)
}

// commentTextFmt formats a source code comment as text.
func commentTextFmt(v string) string {
	const indent = "    "
	var buf bytes.Buffer
	godoc.ToText(&buf, v, indent, "\t", 80-2*len(indent))
	p := buf.Bytes()
	return string(p)
}

// declFmt formats a Decl as HTML.
func declFmt(decl doc.Decl) string {
	var buf bytes.Buffer
	last := 0
	t := []byte(decl.Text)
	for _, a := range decl.Annotations {
		p := a.ImportPath
		if p != "" {
			p = "/" + p
		}
		template.HTMLEscape(&buf, t[last:a.Pos])
		buf.WriteString(`<a href="`)
		buf.WriteString(urlFmt(p))
		buf.WriteByte('#')
		buf.WriteString(urlFmt(a.Name))
		buf.WriteString(`">`)
		template.HTMLEscape(&buf, t[a.Pos:a.End])
		buf.WriteString(`</a>`)
		last = a.End
	}
	template.HTMLEscape(&buf, t[last:])
	return buf.String()
}

func commandNameFmt(pdoc *doc.Package) string {
	_, name := path.Split(pdoc.ImportPath)
	return template.HTMLEscapeString(name)
}

func breadcrumbsFmt(pdoc *doc.Package) string {
	importPath := []byte(pdoc.ImportPath)
	var buf bytes.Buffer
	i := 0
	j := len(pdoc.ProjectRoot)
	switch {
	case j == 0:
		buf.WriteString("<a href=\"/-/go\" title=\"Standard Packages\">☆</a> ")
		j = bytes.IndexByte(importPath, '/')
	case j >= len(importPath):
		j = -1
	}
	for j > 0 {
		buf.WriteString(`<a href="/`)
		buf.WriteString(urlFmt(string(importPath[:i+j])))
		buf.WriteString(`">`)
		template.HTMLEscape(&buf, importPath[i:i+j])
		buf.WriteString(`</a>/`)
		i = i + j + 1
		j = bytes.IndexByte(importPath[i:], '/')
	}
	template.HTMLEscape(&buf, importPath[i:])
	return buf.String()
}

func urlFmt(path string) string {
	u := url.URL{Path: path}
	return u.String()
}

type texample struct {
	Object  interface{}
	Example *doc.Example
}

func examples(pdoc *doc.Package) (examples []*texample) {
	for _, e := range pdoc.Examples {
		examples = append(examples, &texample{pdoc, e})
	}
	for _, f := range pdoc.Funcs {
		for _, e := range f.Examples {
			examples = append(examples, &texample{f, e})
		}
	}
	for _, t := range pdoc.Types {
		for _, e := range t.Examples {
			examples = append(examples, &texample{t, e})
		}
		for _, f := range t.Funcs {
			for _, e := range f.Examples {
				examples = append(examples, &texample{f, e})
			}
		}
		for _, f := range t.Methods {
			for _, e := range f.Examples {
				examples = append(examples, &texample{f, e})
			}
		}
	}
	return
}

func exampleIdFmt(v interface{}, example *doc.Example) string {
	buf := make([]byte, 0, 64)
	buf = append(buf, "_example"...)

	switch v := v.(type) {
	case *doc.Type:
		buf = append(buf, '_')
		buf = append(buf, v.Name...)
	case *doc.Func:
		buf = append(buf, '_')
		if v.Recv != "" {
			if v.Recv[0] == '*' {
				buf = append(buf, v.Recv[1:]...)
			} else {
				buf = append(buf, v.Recv...)
			}
			buf = append(buf, '_')
		}
		buf = append(buf, v.Name...)
	}
	if example.Name != "" {
		buf = append(buf, '-')
		buf = append(buf, example.Name...)
	}
	return template.HTMLEscapeString(string(buf))
}

var contentTypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".txt":  "text/plain; charset=utf-8",
}

func executeTemplate(w http.ResponseWriter, name string, status int, data interface{}) error {
	s := templateSet
	if appengine.IsDevAppServer() {
		var err error
		s, err = parseTemplates()
		if err != nil {
			return err
		}
	}
	if ct, ok := contentTypes[path.Ext(name)]; ok {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(status)
	return s.ExecuteTemplate(w, name, data)
}

var templateSet *template.Template

func parseTemplates() (*template.Template, error) {
	// Is there a better way to call ParseGlob with application specified
	// funcs? The dummy template thing is gross.
	set, err := template.New("__dummy__").Parse(`{{define "__dummy__"}}{{end}}`)
	if err != nil {
		return nil, err
	}
	set.Funcs(template.FuncMap{
		"comment":      commentFmt,
		"commentText":  commentTextFmt,
		"decl":         declFmt,
		"equal":        reflect.DeepEqual,
		"map":          mapFmt,
		"breadcrumbs":  breadcrumbsFmt,
		"commandName":  commandNameFmt,
		"relativePath": relativePathFmt,
		"relativeTime": relativeTime,
		"importPath":   importPathFmt,
		"url":          urlFmt,
		"exampleId":    exampleIdFmt,
		"examples":     examples,
		"isType":       func(v interface{}) bool { _, ok := v.(*doc.Type); return ok },
		"isPackage":    func(v interface{}) bool { _, ok := v.(*doc.Package); return ok },
		"isFunc":       func(v interface{}) bool { _, ok := v.(*doc.Func); return ok },
	})
	_, err = set.ParseGlob("template/*.html")
	if err != nil {
		return nil, err
	}
	return set.ParseGlob("template/*.txt")
}

func init() {
	templateSet = template.Must(parseTemplates())
}
