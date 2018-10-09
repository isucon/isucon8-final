package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func main() {
	var (
		dsn = flag.String("dsn", "root:root@tcp(127.0.0.1:3306)/isucoin", "mysql dsn")
	)
	flag.Parse()
	db, err := sqlx.Open("mysql", *dsn+"?parseTime=true&loc=Local&charset=utf8mb4")
	if err != nil {
		log.Fatal(err)
	}
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Panicln(err)
	}
	time.Local = loc
	if err = run(db); err != nil {
		log.Fatal(err)
	}
}

func run(db *sqlx.DB) error {
	// copy from InitBenchmark
	stop := time.Now()
	if stop.Hour() >= 10 {
		stop = time.Date(stop.Year(), stop.Month(), stop.Day(), 10, 0, 0, 0, stop.Location())
	} else {
		stop = time.Date(stop.Year(), stop.Month(), stop.Day()-1, 10, 0, 0, 0, stop.Location())
	}
	// ISUCON開催時間を初期セットしている
	deftime := time.Date(2018, 10, 20, 10, 0, 0, 0, stop.Location())
	diff := deftime.Sub(stop)
	diffs := int64(diff.Seconds())
	// TODO よく考えたらupdateとalter table drop/add partitionだけで行ける気がする
	// 複製を作成
	db.MustExec("DROP TABLE IF EXISTS orders_new")
	db.MustExec("DROP TABLE IF EXISTS trade_new")
	db.MustExec("CREATE TABLE orders_new LIKE orders")
	db.MustExec("CREATE TABLE trade_new LIKE trade")

	type Partition struct {
		PartitionName string `db:"PARTITION_NAME"`
	}

	addPartition := func(table string) error {
		partitions := []Partition{}
		if err := db.Select(&partitions, "SELECT PARTITION_NAME FROM INFORMATION_SCHEMA.PARTITIONS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", table); err != nil {
			return err
		}
		q := fmt.Sprintf("ALTER TABLE %s PARTITION BY RANGE COLUMNS(created_at) (", table)
		for _, partition := range partitions {
			if partition.PartitionName == "pmax" {
				continue
			}
			d, err := time.Parse("p2006010215", partition.PartitionName)
			if err != nil {
				return err
			}
			// diffを引く
			t := d.Add(-diff)
			q += fmt.Sprintf("PARTITION %s VALUES LESS THAN ('%s'),", t.Format("p2006010215"), t.Format("2006-01-02 15:00:00"))
		}
		q += "PARTITION pmax VALUES LESS THAN MAXVALUE);"
		_, err := db.Exec(q)
		return err
	}
	if err := addPartition("orders_new"); err != nil {
		return err
	}
	if err := addPartition("trade_new"); err != nil {
		return err
	}
	if _, err := db.Exec("INSERT INTO orders_new SELECT id, type, user_id, amount, price, closed_at - INTERVAL ? SECOND, trade_id, created_at - INTERVAL ? SECOND FROM orders", diffs, diffs); err != nil {
		return err
	}
	if _, err := db.Exec("INSERT INTO trade_new SELECT id, amount, price, created_at - INTERVAL ? SECOND FROM trade", diffs); err != nil {
		return err
	}
	db.MustExec("drop table orders")
	db.MustExec("rename table orders_new to orders")

	db.MustExec("drop table trade")
	db.MustExec("rename table trade_new to trade")

	db.MustExec("update user set created_at = created_at - INTERVAL ? SECOND", diffs)

	return nil
}
