package model

import (
	"isucon8/isubank"
	"isucon8/isulogger"
	"log"

	"github.com/pkg/errors"
)

const (
	BankEndpoint = "bank_endpoint"
	BankAppid    = "bank_appid"
	LogEndpoint  = "log_endpoint"
	LogAppid     = "log_appid"
)

func SetSetting(d QueryExecuter, k, v string) error {
	_, err := d.Exec(`INSERT INTO setting (name, val) VALUES (?, ?) ON DUPLICATE KEY UPDATE val = VALUES(val)`, k, v)
	return err
}

func GetSetting(d QueryExecuter, k string) (v string, err error) {
	err = d.QueryRow(`SELECT val FROM setting WHERE name = ?`, k).Scan(&v)
	return
}

func Isubank(d QueryExecuter) (*isubank.Isubank, error) {
	ep, err := GetSetting(d, BankEndpoint)
	if err != nil {
		return nil, errors.Wrapf(err, "getSetting failed. %s", BankEndpoint)
	}
	id, err := GetSetting(d, BankAppid)
	if err != nil {
		return nil, errors.Wrapf(err, "getSetting failed. %s", BankAppid)
	}
	return isubank.NewIsubank(ep, id)
}

func Logger(d QueryExecuter) (*isulogger.Isulogger, error) {
	ep, err := GetSetting(d, LogEndpoint)
	if err != nil {
		return nil, errors.Wrapf(err, "getSetting failed. %s", LogEndpoint)
	}
	id, err := GetSetting(d, LogAppid)
	if err != nil {
		return nil, errors.Wrapf(err, "getSetting failed. %s", LogAppid)
	}
	return isulogger.NewIsulogger(ep, id)
}

func sendLog(d QueryExecuter, tag string, v interface{}) {
	logger, err := Logger(d)
	if err != nil {
		log.Printf("[WARN] new logger failed. tag: %s, v: %v, err:%s", tag, v, err)
		return
	}
	err = logger.Send(tag, v)
	if err != nil {
		log.Printf("[WARN] logger send failed. tag: %s, v: %v, err:%s", tag, v, err)
	}
}
