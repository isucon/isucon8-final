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

	"github.com/hpcloud/tail"
	"bench/portal"
	"github.com/pkg/errors"
)

var (
	errNoJob   = errors.New("No task")
	hostname   = "unknown"
	pathPrefix = "bench/"

	portalUrl = flag.String("portal", "https://portal."+portal.Domain, "portal host")
	tempDir   = flag.String("tempdir", "", "path to temp dir")
	benchcmd  = flag.String("bench", "bench", "path to benchmark command")
	wsPort    = flag.Int("wsPort", 15873, "port of websocket server")
	domain    = flag.String("domain", ".isucon8.flying-chair.net", "domain name")
)

func main() {
	flag.Parse()

	portal := strings.TrimSuffix(*portalUrl, "/")
	run(*tempDir, portal)
}

func updateHostname() {
	name, err := os.Hostname()
	if err == nil {
		hostname = name + *domain
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
		if logPath != "" {
			if file, err := os.Open(logPath); err == nil {
				part, _ := writer.CreateFormFile("log", filepath.Base(logPath))
				io.Copy(part, file)
				file.Close()
			}
		}

		writer.Close()

		u, err := getUrl("/" + pathPrefix + "job/result")
		if err != nil {
			return err
		}

		q := u.Query()
		q.Set("job_id", fmt.Sprint(job.ID))
		if aborted {
			q.Set("aborted", "yes")
		}
		u.RawQuery = q.Encode()

		log.Printf("[INFO] send result %s", u.String())
		req, err := http.NewRequest("POST", u.String(), body)
		if err != nil {
			return errors.Wrap(err, "http.NewRequest failed")
		}

		req.Header.Set("Content-Type", writer.FormDataContentType())

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return errors.Wrap(err, "request failed")
		}

		defer res.Body.Close()
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return errors.Wrap(err, "ioutil.ReadAll")
		}
		if res.StatusCode >= 400 {
			return errors.Errorf("status code is not success. code: %d, body: %s", res.StatusCode, string(b))
		}

		log.Println(string(b))
		return nil
	}

	messageCh := startWS(*wsPort)
	for {
		job := getJobLoop()
		now := time.Now()
		rname := fmt.Sprintf("isucon8f-benchresult-%d-%d.json", now.Unix(), job.ID)
		lname := fmt.Sprintf("isucon8f-benchlog-%d-%d.log", now.Unix(), job.ID)
		tname := fmt.Sprintf("isucon8f-stdout-%d-%d.log", now.Unix(), job.ID)
		result := path.Join(tempDir, rname)
		logpath := path.Join(tempDir, lname)
		teepath := path.Join(tempDir, tname)
		statepath := path.Join(tempDir, "laststate.json") // 最終的な試験のとき指定すればよいが面倒なので毎回更新する
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
		args = append(args, fmt.Sprintf("-teestdout=%s", teepath))
		args = append(args, fmt.Sprintf("-stateout=%s", statepath))

		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, *benchcmd, args...)

		tailCh := make(chan struct{})
		go func() {
			t, err := tail.TailFile(teepath, tail.Config{Follow: true})
			if err != nil {
				log.Println(err.Error())
				return
			}
			for {
				select {
				case line := <-t.Lines:
					if line.Err != nil {
						log.Println(line.Err.Error())
						return
					}
					messageCh <- logMessage{
						jobID: job.ID,
						text:  line.Text,
					}
				case <-tailCh:
					messageCh <- logMessage{
						jobID:    job.ID,
						finished: true,
					}
					return
				}
			}
		}()

		log.Println("Start benchmark args:", cmd.Args)
		err := cmd.Start()
		if err != nil {
			log.Println(err)
			close(tailCh)
			continue
		}

		err = cmd.Wait()
		if err != nil {
			aborted = true
			log.Println(err)
		}
		close(tailCh)

		for try := 0; try < 3; try++ {
			if err = postResult(job, result, logpath, aborted); err == nil {
				break
			}
			logpath = ""
			log.Printf("failed post result. err: %s, try: %d", err, try)
			time.Sleep(10 * time.Second)
		}
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
