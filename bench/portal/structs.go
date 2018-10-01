package portal

import "time"

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
	BankIP   string `json:"bank_ip"`
	LogIP    string `json:"log_ip"`
}
