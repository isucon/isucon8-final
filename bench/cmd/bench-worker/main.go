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
	"strconv"
	"strings"
	"time"

	"github.com/ken39arg/isucon2018-final/bench"
	"github.com/pkg/errors"
)

var (
	errNoJob   = errors.New("No task")
	hostname   = "unknown"
	pathPrefix = "bench/"

	portalUrl = flag.String("portal", "http://172.18.0.1:3333", "portal host")
	tempDir   = flag.String("temdir", "", "path to temp dir")
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
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-remotes") ||
			strings.HasPrefix(arg, "-output") {
			log.Fatalln("Cannot use the option", arg, "on workermode")
		}
	}

	updateHostname()

	var baseArgs []string
	for _, arg := range os.Args {
		if !strings.HasPrefix(arg, "-workermode") {
			baseArgs = append(baseArgs, arg)
		}
	}

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

	getJob := func() (*Job, error) {
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
		j := new(Job)
		dec := json.NewDecoder(res.Body)
		err = dec.Decode(j)
		if err != nil {
			return nil, err
		}
		return j, nil
	}

	getJobLoop := func() *Job {
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

	postResult := func(job *Job, result BenchResult, l io.Reader, aborted bool) error {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		{
			part, _ := writer.CreateFormFile("result", "resutl.json")
			json.NewEncoder(part).Encode(result)
		}

		{
			part, _ := writer.CreateFormFile("log", "output.log")
			io.Copy(part, l)
		}

		writer.Close()

		u, err := getUrl("/" + pathPrefix + "job/result")
		if err != nil {
			return err
		}

		q := u.Query()
		q.Set("job_id", fmt.Sprint(job.ID))
		if aborted {
			q.Set("is_aborted", "1")
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
		out := &bytes.Buffer{}
		result := func() BenchResult {
			result := BenchResult{
				JobID:     strconv.Itoa(job.ID),
				IPAddrs:   job.TargetIP,
				StartTime: time.Now(),
			}
			var (
				// TODO
				bankEndpoint           string
				loggerEndpoint         string
				bankInternalEndpoint   string
				loggerInternalEndpoint string
			)
			mgr, err := bench.NewManager(out, "http://"+job.TargetIP, bankEndpoint, loggerEndpoint, bankInternalEndpoint, loggerInternalEndpoint)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("benchmarker の初期化に失敗しました. err: %s", err))
				result.Message = "システムエラーです。運営に連絡してください"
				return result
			}
			defer mgr.Close()
			bm := bench.NewRunner(mgr)
			if err = bm.Run(context.Background()); err != nil {
				result.Errors = append(result.Errors, err.Error())
			}
			bm.Result()

			result.Score = mgr.TotalScore()
			result.Pass = 0 < result.Score
			result.LoadLevel = int(mgr.GetLevel())
			// TODO
			// result.Logs
			if result.Pass {
				result.Message = "Success"
			} else {
				result.Message = "Failed"
			}
			return result
		}()
		result.EndTime = time.Now()
		aborted := false // TODO

		if err := postResult(job, result, out, aborted); err != nil {
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
