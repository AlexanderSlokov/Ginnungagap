package pypi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/html"
)

var errNotFound = errors.New("not found upstream")

// UpstreamFile is one file of an upstream project page.
type UpstreamFile struct {
	Filename       string
	URL            string
	SHA256         string
	RequiresPython string
	Yanked         *string // nil: not yanked
	UploadTime     *time.Time
	Size           *int64
	MetadataSHA256 string // PEP 658 core metadata; empty if not available
}

var hexSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// NewClient returns an HTTP client that does not connect to loopback, private,
// or link-local addresses. A file URL comes from the upstream index, so an upstream
// must not make the proxy send requests to the internal network.
func NewClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
				ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
				return fmt.Errorf("upstream address %s is not allowed", host)
			}
			return nil
		},
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	tr.DialContext = dialer.DialContext
	tr.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{Transport: tr}
}

const upstreamAccept = "application/vnd.pypi.simple.v1+json, application/vnd.pypi.simple.v1+html;q=0.2, text/html;q=0.01"

// maxIndexSize limits the size of one upstream project page.
const maxIndexSize = 64 << 20

// FetchProject reads the project page of name from an upstream Simple API.
func FetchProject(ctx context.Context, c *http.Client, base, name string) ([]UpstreamFile, error) {
	page, err := url.Parse(strings.TrimSuffix(base, "/") + "/" + url.PathEscape(name) + "/")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, page.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", upstreamAccept)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch {
	// An index on S3 (for example, download.pytorch.org) returns 403 for a missing project.
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusGone:
		return nil, errNotFound
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("upstream %s: %s", page, resp.Status)
	}
	body := io.LimitReader(resp.Body, maxIndexSize)
	if strings.Contains(resp.Header.Get("Content-Type"), "json") {
		return parseJSON(body, resp.Request.URL)
	}
	return parseHTML(body, resp.Request.URL)
}

func parseTime(s string) *time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return nil
	}
	t = t.UTC()
	return &t
}

// metadataHash reads a PEP 658 value: "sha256=<hex>" in HTML, or {"sha256": "<hex>"} in JSON.
func metadataHash(v any) string {
	switch m := v.(type) {
	case string:
		if h, ok := strings.CutPrefix(m, "sha256="); ok && hexSHA256.MatchString(h) {
			return h
		}
	case map[string]any:
		if h, ok := m["sha256"].(string); ok && hexSHA256.MatchString(h) {
			return h
		}
	}
	return ""
}

// accept checks and completes a file. It returns false for a file that the proxy cannot verify.
func accept(f *UpstreamFile, page *url.URL, href string) bool {
	u, err := page.Parse(href)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return false
	}
	if h, ok := strings.CutPrefix(u.Fragment, "sha256="); ok && f.SHA256 == "" {
		f.SHA256 = h
	}
	u.Fragment = ""
	f.URL = u.String()
	if f.Filename == "" {
		f.Filename, _ = url.PathUnescape(u.Path[strings.LastIndex(u.Path, "/")+1:])
	}
	f.SHA256 = strings.ToLower(f.SHA256)
	return hexSHA256.MatchString(f.SHA256) && f.Filename != "" &&
		!strings.ContainsAny(f.Filename, "/\\") && Version(f.Filename) != ""
}

func parseJSON(r io.Reader, page *url.URL) ([]UpstreamFile, error) {
	var doc struct {
		Files []struct {
			Filename       string            `json:"filename"`
			URL            string            `json:"url"`
			Hashes         map[string]string `json:"hashes"`
			RequiresPython *string           `json:"requires-python"`
			Yanked         any               `json:"yanked"`
			UploadTime     string            `json:"upload-time"`
			Size           *int64            `json:"size"`
			CoreMetadata   any               `json:"core-metadata"`
			DistInfo       any               `json:"data-dist-info-metadata"`
		} `json:"files"`
	}
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return nil, err
	}
	var out []UpstreamFile
	for _, jf := range doc.Files {
		f := UpstreamFile{Filename: jf.Filename, SHA256: jf.Hashes["sha256"], Size: jf.Size, UploadTime: parseTime(jf.UploadTime)}
		if jf.RequiresPython != nil {
			f.RequiresPython = *jf.RequiresPython
		}
		switch y := jf.Yanked.(type) {
		case bool:
			if y {
				f.Yanked = new(string)
			}
		case string:
			f.Yanked = &y
		}
		if f.MetadataSHA256 = metadataHash(jf.CoreMetadata); f.MetadataSHA256 == "" {
			f.MetadataSHA256 = metadataHash(jf.DistInfo)
		}
		if accept(&f, page, jf.URL) {
			out = append(out, f)
		}
	}
	return out, nil
}

func parseHTML(r io.Reader, page *url.URL) ([]UpstreamFile, error) {
	z := html.NewTokenizer(r)
	var out []UpstreamFile
	var cur *UpstreamFile
	var href string
	for {
		switch z.Next() {
		case html.ErrorToken:
			if errors.Is(z.Err(), io.EOF) {
				return out, nil
			}
			return nil, z.Err()
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			switch t.Data {
			case "base":
				for _, a := range t.Attr {
					if a.Key == "href" {
						if u, err := page.Parse(a.Val); err == nil {
							page = u
						}
					}
				}
			case "a":
				cur, href = &UpstreamFile{}, ""
				for _, a := range t.Attr {
					switch a.Key {
					case "href":
						href = a.Val
					case "data-requires-python":
						cur.RequiresPython = a.Val
					case "data-yanked":
						y := a.Val
						cur.Yanked = &y
					case "data-core-metadata":
						cur.MetadataSHA256 = metadataHash(a.Val)
					case "data-dist-info-metadata":
						if cur.MetadataSHA256 == "" {
							cur.MetadataSHA256 = metadataHash(a.Val)
						}
					case "data-upload-time":
						cur.UploadTime = parseTime(a.Val)
					}
				}
			}
		case html.TextToken:
			if cur != nil {
				cur.Filename += strings.TrimSpace(string(z.Text()))
			}
		case html.EndTagToken:
			if t := z.Token(); t.Data == "a" && cur != nil {
				if accept(cur, page, href) {
					out = append(out, *cur)
				}
				cur = nil
			}
		}
	}
}
