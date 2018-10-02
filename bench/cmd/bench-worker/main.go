package main

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ken39arg/isucon2018-final/bench/portal"
	"github.com/pkg/errors"
)

var (
	errNoJob   = errors.New("No task")
	hostname   = "unknown"
	pathPrefix = "bench/"

	portalUrl = flag.String("portal", "https://portal."+portal.Domain, "portal host")
	tempDir   = flag.String("temdir", "", "path to temp dir")
	benchcmd  = flag.String("bench", "bench", "path to temp dir")
)

func main() {
	flag.Parse()

	portal := strings.TrimSuffix(*portalUrl, "/")
	run(*tempDir, portal)
}

func updateHostname() {
	name, err := os.Hostname()
	if err == nil {
		hostname = name
	}
}

func run(tempDir, portalUrl string) {
	updateHostname()

	getUrl := func(path string) (*url.URL, error) {
		u, err := url.Parse(portalUrl + path)
		if err != nil {
			return nil, err
		}
		if u.Scheme == "" {
			u.Scheme = "http"
		}
		return u, nil
	}

	getJob := func() (*portal.Job, error) {
		u, err := getUrl("/" + pathPrefix + "job")
		if err != nil {
			return nil, err
		}

		q := u.Query()
		q.Set("hostname", hostname)
		u.RawQuery = q.Encode()
		res, err := http.Get(u.String())
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		if res.StatusCode == http.StatusNoContent {
			return nil, errNoJob
		}
		j := new(portal.Job)
		dec := json.NewDecoder(res.Body)
		err = dec.Decode(j)
		if err != nil {
			return nil, err
		}
		err = j.Setup()
		if err != nil {
			return nil, err
		}
		return j, nil
	}

	getJobLoop := func() *portal.Job {
		for {
			task, err := getJob()
			if err == nil {
				return task
			}

			log.Println(err)
			if err == errNoJob {
				time.Sleep(5 * time.Second)
			} else {
				time.Sleep(30 * time.Second)
			}
		}
	}

	postResult := func(job *portal.Job, jsonPath string, logPath string, aborted bool) error {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		if file, err := os.Open(jsonPath); err == nil {
			part, _ := writer.CreateFormFile("result", filepath.Base(jsonPath))
			io.Copy(part, file)
			file.Close()
		}

		if file, err := os.Open(logPath); err == nil {
			part, _ := writer.CreateFormFile("log", filepath.Base(logPath))
			io.Copy(part, file)
			file.Close()
		}

		writer.Close()

		u, err := getUrl("/" + pathPrefix + "job/result")
		if err != nil {
			return err
		}

		q := u.Query()
		q.Set("jobid", fmt.Sprint(job.ID))
		if aborted {
			q.Set("aborted", "yes")
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequest("POST", u.String(), body)
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", writer.FormDataContentType())

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}

		defer res.Body.Close()
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}

		log.Println(string(b))
		return nil
	}

	for {
		job := getJobLoop()
		now := time.Now()
		rname := fmt.Sprintf("isucon8f-benchresult-%d-%d.json", now.Unix(), job.ID)
		lname := fmt.Sprintf("isucon8f-benchlog-%d-%d.log", now.Unix(), job.ID)
		result := path.Join(tempDir, rname)
		logpath := path.Join(tempDir, lname)
		aborted := false

		var args []string
		args = append(args, fmt.Sprintf("-jobid=%d", job.ID))
		args = append(args, fmt.Sprintf("-appep=%s", job.TargetURL))
		args = append(args, fmt.Sprintf("-bankep=%s", job.BankURL))
		args = append(args, fmt.Sprintf("-logep=%s", job.LogURL))
		args = append(args, fmt.Sprintf("-internalbank=%s", job.InternalBankURL))
		args = append(args, fmt.Sprintf("-internallog=%s", job.InternalLogURL))
		args = append(args, fmt.Sprintf("-result=%s", result))
		args = append(args, fmt.Sprintf("-log=%s", logpath))

		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, *benchcmd, args...)

		log.Println("Start benchmark args:", cmd.Args)
		err := cmd.Start()
		if err != nil {
			log.Println(err)
			continue
		}

		err = cmd.Wait()
		if err != nil {
			aborted = true
			log.Println(err)
		}

		err = postResult(job, result, logpath, aborted)
		if err != nil {
			log.Println(err)
		}

		time.Sleep(time.Second)
	}
}

func init() {
	var s int64
	if err := binary.Read(crand.Reader, binary.LittleEndian, &s); err != nil {
		s = time.Now().UnixNano()
	}
	rand.Seed(s)
}
