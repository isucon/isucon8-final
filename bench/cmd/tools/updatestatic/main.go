package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"go/format"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

var (
	staticDir string
	benchDir  string
)

func init() {
	flag.StringVar(&benchDir, "benchdir", "bench", "path to bench/src/bench directory")
	flag.StringVar(&staticDir, "staticdir", "webapp/public", "path to webapp/public directory")
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

type hasherTransport struct {
	targetHost string
	t          http.RoundTripper
}

func (ct *hasherTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Host
	defer func() {
		req.URL.Host = host
	}()

	req.URL.Host = ct.targetHost
	res, err := ct.t.RoundTrip(req)
	return res, err
}

type TemplateArg struct {
	StaticFiles []*StaticFile
}

type StaticFile struct {
	Path string
	Size int64
	Hash string
}

const staticFileTemplate = `
package bench

type StaticFile struct {
	Path string
	Size int64
	Hash string
}

var StaticFiles = []*StaticFile {
{{ range .StaticFiles }} &StaticFile { "{{ .Path }}", {{ .Size }}, "{{ .Hash }}" },
{{ end }}
}
`

func prepareStaticFiles() []*StaticFile {
	var ret []*StaticFile
	err := filepath.Walk(staticDir, func(path string, info os.FileInfo, err error) error {
		must(err)
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".map") {
			return nil
		}

		subPath := path[len(staticDir):]
		subPath = strings.TrimSuffix(subPath, "index.html")

		f, err := os.Open(path)
		must(err)
		defer f.Close()

		h := md5.New()
		_, err = io.Copy(h, f)
		must(err)

		hash := hex.EncodeToString(h.Sum(nil))

		ret = append(ret, &StaticFile{
			Path: subPath,
			Size: info.Size(),
			Hash: hash,
		})

		return nil
	})
	must(err)

	// canonicalize
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Path < ret[j].Path
	})
	return ret
}

func writeStaticFileGo() {
	const saveName = "staticfile.go"
	files := prepareStaticFiles()

	t := template.Must(template.New(saveName).Parse(staticFileTemplate))

	var buf bytes.Buffer
	t.Execute(&buf, TemplateArg{
		StaticFiles: files,
	})

	fmt.Print(buf.String())

	data, err := format.Source(buf.Bytes())
	must(err)

	err = ioutil.WriteFile(path.Join(benchDir, saveName), data, 0644)
	must(err)

	log.Println("save", saveName)
}

func main() {
	flag.Parse()
	var err error
	staticDir, err = filepath.Abs(staticDir)
	must(err)

	benchDir, err = filepath.Abs(benchDir)
	must(err)

	writeStaticFileGo()
}
