package bench

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
)

type FinalState struct {
	BaseURL string
	BankID  string
	Name    string
	Pass    string
	Orders  []Order
	Info    *InfoResponse
}

func (s *FinalState) Check(ctx context.Context) error {
	client, err := NewClient(s.BaseURL, s.BankID, s.Name, s.Pass, ClientTimeout, RetireTimeout)
	if err != nil {
		return errors.Wrap(err, "NewClient failed")
	}
	if err = client.Signin(ctx); err != nil {
		return errors.Wrap(err, "ログインできません")
	}
	info, err := client.Info(ctx, 0)
	if err != nil {
		return errors.Wrap(err, "GET /infoを取得できません")
	}
	chartTest := func(expect, got []CandlestickData) bool {
		e := expect[:len(expect)-2]
		g := got[:len(expect)-2]
		return reflect.DeepEqual(e, g)
	}
	if !chartTest(s.Info.ChartBySec, info.ChartBySec) {
		return errors.Errorf("ChartBySec unmatch")
	}
	if !chartTest(s.Info.ChartByMin, info.ChartByMin) {
		return errors.Errorf("ChartByMin unmatch")
	}
	if !chartTest(s.Info.ChartByHour, info.ChartByHour) {
		return errors.Errorf("ChartByHour unmatch")
	}
	orders, err := client.GetOrders(ctx)
	if err != nil {
		return errors.Wrap(err, "GET /ordersを取得できません")
	}
	if !reflect.DeepEqual(orders, s.Orders) {
		return errors.Errorf("Orders unmatch")
	}
	return nil
}
