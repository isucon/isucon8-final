package portal

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const Domain = "isucon8.flying-chair.net"

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
	ID       int    `json:"id"`
	TeamID   int    `json:"team_id"`
	TargetIP string `json:"target_ip"`

	TargetURL       string `json:"target_url"`
	BankURL         string `json:"bank_url"`
	LogURL          string `json:"log_url"`
	InternalBankURL string `json:"internal_bank_url"`
	InternalLogURL  string `json:"internal_log_url"`
}

func (j *Job) Setup() error {
	octets := strings.SplitN(j.TargetIP, ".", 4)
	if len(octets) != 4 {
		return errors.Errorf("invalid IPv4 address: %s", j.TargetIP)
	}
	_team, num := octets[2], octets[3]
	team, err := strconv.ParseInt(_team, 10, 64)
	if err != nil {
		return errors.Errorf("invalid team id %s", _team)
	}
	team = team - 1 // bench subnet - 1
	j.TargetURL = fmt.Sprintf("https://b%s-%d.%s", num, team, Domain)
	j.BankURL = fmt.Sprintf("https://bank-%d.%s", team, Domain)
	j.LogURL = fmt.Sprintf("https://logger-%d.%s", team, Domain)
	j.InternalBankURL = fmt.Sprintf("https://bank-%d.%s", team, Domain)
	j.InternalLogURL = fmt.Sprintf("https://loggerp-%d.%s", team, Domain)
	return nil
}
