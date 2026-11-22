package pypi

import (
	"net/url"
	"strings"
	"testing"
)

const sha = "7b6dc07cfc2ad7170c2f4c6b87ec414fadbf306420352f643597fd222f465fdf"
const meta = "7f56e1b49211bfcfe45a8c7df42d96a054f69a96fbf6bc4f1b6b6fe8d3f097a7"

func TestNormalizeAndVersion(t *testing.T) {
	for in, want := range map[string]string{"Django": "django", "zope.interface": "zope-interface", "Foo__Bar-.baz": "foo-bar-baz"} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q", in, got)
		}
	}
	for in, want := range map[string]string{
		"torch-2.10.0+cu128-cp310-cp310-manylinux_2_28_aarch64.whl": "2.10.0+cu128",
		"six-1.16.0-py2.py3-none-any.whl":                           "1.16.0",
		"pkg-1.0-1-py3-none-any.whl":                                "1.0",
		"mpmath-0.1.zip":                                            "0.1",
		"python-dateutil-2.9.0.tar.gz":                              "2.9.0",
		"bad.whl":                                                   "",
	} {
		if got := Version(in); got != want {
			t.Errorf("Version(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseHTML(t *testing.T) {
	page, _ := url.Parse("https://download.pytorch.org/whl/cu128/torch/")
	doc := `<html><body>
<a href="https://download-r2.pytorch.org/whl/cu128/torch-2.10.0%2Bcu128-cp310-cp310-manylinux_2_28_aarch64.whl#sha256=` + sha + `" data-dist-info-metadata="sha256=` + meta + `" data-core-metadata="sha256=` + meta + `" data-upload-time="2026-01-21T15:21:34Z" data-requires-python="&gt;=3.10">torch-2.10.0+cu128-cp310-cp310-manylinux_2_28_aarch64.whl</a><br/>
<a href="../../files/six-1.0.tar.gz#sha256=` + strings.ToUpper(sha) + `" data-yanked="">six-1.0.tar.gz</a>
<a href="nohash-1.0.tar.gz">nohash-1.0.tar.gz</a>
<a href="file:///etc/passwd#sha256=` + sha + `">x-1.0.tar.gz</a>
</body></html>`
	files, err := parseHTML(strings.NewReader(doc), page)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2: %+v", len(files), files)
	}
	f := files[0]
	if f.Filename != "torch-2.10.0+cu128-cp310-cp310-manylinux_2_28_aarch64.whl" || f.SHA256 != sha || f.MetadataSHA256 != meta ||
		f.RequiresPython != ">=3.10" || f.UploadTime == nil || f.UploadTime.Year() != 2026 || f.Yanked != nil ||
		f.URL != "https://download-r2.pytorch.org/whl/cu128/torch-2.10.0%2Bcu128-cp310-cp310-manylinux_2_28_aarch64.whl" {
		t.Errorf("file 0 = %+v", f)
	}
	f = files[1]
	if f.URL != "https://download.pytorch.org/whl/files/six-1.0.tar.gz" || f.SHA256 != sha || f.Yanked == nil || *f.Yanked != "" {
		t.Errorf("file 1 = %+v", f)
	}
}

func TestParseJSON(t *testing.T) {
	page, _ := url.Parse("https://pypi.org/simple/six/")
	doc := `{"meta":{"api-version":"1.4"},"name":"six","files":[
{"filename":"six-1.16.0-py2.py3-none-any.whl","url":"https://files.pythonhosted.org/packages/six-1.16.0-py2.py3-none-any.whl","hashes":{"sha256":"` + sha + `"},"requires-python":">=2.7","yanked":false,"upload-time":"2021-05-05T14:18:17.237154Z","size":11053,"core-metadata":{"sha256":"` + meta + `"}},
{"filename":"six-1.0.0.tar.gz","url":"/packages/six-1.0.0.tar.gz","hashes":{"sha256":"` + sha + `"},"yanked":"broken","core-metadata":true},
{"filename":"six-0.1.tar.gz","url":"/packages/six-0.1.tar.gz","hashes":{"md5":"abc"}}]}`
	files, err := parseJSON(strings.NewReader(doc), page)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	f := files[0]
	if f.RequiresPython != ">=2.7" || f.Yanked != nil || f.Size == nil || *f.Size != 11053 || f.MetadataSHA256 != meta || f.UploadTime == nil {
		t.Errorf("file 0 = %+v", f)
	}
	f = files[1]
	if f.URL != "https://pypi.org/packages/six-1.0.0.tar.gz" || f.Yanked == nil || *f.Yanked != "broken" || f.MetadataSHA256 != "" {
		t.Errorf("file 1 = %+v", f)
	}
}
