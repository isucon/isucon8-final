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

	"github.com/kayac/inhouse-isucon-2018/bench"
	"github.com/pkg/errors"
)

var (
	//groupName  = os.Getenv("ISU7_GROUP_NAME")
	groupName  = "1"
	errNoJob   = errors.Errorf("No task: %s", groupName)
	nodeName   = "unknown"
	portalUrl  = flag.String("portal", "http://172.18.0.1:3333", "portal host")
	pathPrefix = flag.String("prefix", "kayacno-isuisuconconisuconcon/", "path prefix")
)

type BenchResult struct {
	JobID   string `json:"job_id"`
	IPAddrs string `json:"ip_addrs"`

	Pass      bool     `json:"pass"`
	Score     int64    `json:"score"`
	Message   string   `json:"message"`
	Errors    []string `json:"error"`
	Logs      []string `json:"log"`
	LoadLevel int      `json:"load_level"`

	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

type Job struct {
	ID      int    `json:"id"`
	TeamID  int    `json:"team_id"`
	IPAddrs string `json:"ip_addrs"`
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() (err error) {
	*portalUrl = strings.TrimSuffix(*portalUrl, "/")

	nodeName, err = os.Hostname()
	if err != nil {
		return errors.Wrap(err, "cannnot get Hostname()")
	}

	getUrl := func(path string) (*url.URL, error) {
		u, err := url.Parse(*portalUrl + path)
		if err != nil {
			return nil, err
		}
		if u.Scheme == "" {
			u.Scheme = "http"
		}
		return u, nil
	}

	getJob := func() (*Job, error) {
		u, err := getUrl("/" + *pathPrefix + "job/new")
		if err != nil {
			return nil, err
		}

		res, err := http.PostForm(u.String(), url.Values{"bench_node": {nodeName}, "bench_group": {groupName}})
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

	postResult := func(job *Job, result BenchResult, l io.Reader) error {
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

		u, err := getUrl("/" + *pathPrefix + "job/result")
		if err != nil {
			return err
		}

		q := u.Query()
		q.Set("jobid", fmt.Sprint(job.ID))
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
				IPAddrs:   job.IPAddrs,
				StartTime: time.Now(),
			}
			bm, err := bench.NewBenchmarker(out, bench.BenchmarkerParams{
				Domain: "http://" + job.IPAddrs,
				Time:   time.Minute,
			})
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("benchmarker の初期化に失敗しました. err: %s", err))
				result.Message = "システムエラーです。運営に連絡してください"
				return result
			}
			if err = bm.Run(context.Background()); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("benchmark が正常に終了しませんでした. err: %s", err))
				result.Message = "システムエラーです。運営に連絡してください"
				return result
			}
			result.Pass = 0 < bm.Score()
			result.Score = bm.Score()
			result.LoadLevel = bm.LoadLevel()
			if result.Pass {
				result.Message = "Success"
			} else {
				result.Message = "Failed"
			}
			return result
		}()
		result.EndTime = time.Now()

		err = postResult(job, result, out)
		if err != nil {
			log.Println(err)
		}

		time.Sleep(time.Second)
	}
	return nil
}

func init() {
	var s int64
	if err := binary.Read(crand.Reader, binary.LittleEndian, &s); err != nil {
		s = time.Now().UnixNano()
	}
	rand.Seed(s)
}
