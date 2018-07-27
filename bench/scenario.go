package bench

import (
	"context"
	"math/rand"
	"time"

	"github.com/pkg/errors"
)

const (
	StaticExpectMS = 20.0
	PostExpectMS   = 100.0
	GetExpectMS    = 300.0

	StaticScoreFactor = 5.0
	PostScoreFactor   = 1.2
	GetScoreFactor    = 3.5
)

// それぞれのスコアは (xExpectMS - avgResTimeMS) * xScoreFactor * requestCount
func calcScore(expect, facor, totalms, count float64) float64 {
	if count == 0 {
		return 0
	}
	avg := totalms / count
	p := expect - avg
	if p < 1 {
		p = 1
	}
	//log.Printf("expect: %.5f, factor: %.5f, avg: %.5f count: %.5f", expect, facor, avg, count)
	return p * count * facor
}

type ScenarioBase struct {
	c      *Client
	factor float64
	err    error
}

func (bs *ScenarioBase) WriteError(err error) {
	bs.err = err
}

func (bs *ScenarioBase) Error() error {
	return bs.err
}

func (bs *ScenarioBase) Score() int64 {
	if bs.err != nil && bs.err != context.DeadlineExceeded {
		return 0
	}
	s := calcScore(StaticExpectMS, StaticScoreFactor, bs.c.TotalStaticTime(), float64(bs.c.TotalStaticCount()))
	p := calcScore(PostExpectMS, PostScoreFactor, bs.c.TotalPostTime(), float64(bs.c.TotalPostCount()))
	g := calcScore(GetExpectMS, GetScoreFactor, bs.c.TotalGetAPITime(), float64(bs.c.TotalGetAPICount()))
	return int64((s + p + g) * bs.factor)
}

func NewScenarioBase(base string, httpTimeout time.Duration, factor float64) (*ScenarioBase, error) {
	c, err := NewClient(base, httpTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "New Client failed")
	}
	return &ScenarioBase{c: c, factor: factor}, nil
}

// 最初の1回目 /initializeを叩きresponseのテストをする
type InitializeScenario struct {
	*ScenarioBase
}

func NewInitializeScenario(base string, factor float64) (*InitializeScenario, error) {
	bs, err := NewScenarioBase(base, 10*time.Second, factor)
	if err != nil {
		return nil, err
	}
	return &InitializeScenario{bs}, nil
}

func (s *InitializeScenario) Run(ctx context.Context) error {
	if err := s.c.Static("/initialize", 1); err != nil {
		return errors.Wrap(err, "/initialize に失敗しました")
	}

	// Userを消していないかテスト
	if err := s.c.SetUser("D.Yanasawa", "12345").Login(); err != nil {
		return errors.Wrap(err, "初期化後チェックでユーザーが見つかりませんでした")
	}
	res, err := s.c.Info()
	if err != nil {
		return errors.Wrap(err, "/user にリクエスト失敗")
	}
	if res.User.UserID != "D.Yanasawa" ||
		res.Score.BestScore < 7 ||
		res.Score.TotalScore < 7 ||
		res.Score.TotalRank == 0 ||
		res.Score.BestRank == 0 {
		return errors.Wrap(err, "/user のレスポンスが正しくありません")
	}

	// 最初のユーザーは1点でこれよりスコアの小さいユーザーはいないはず
	if err = s.c.SetUser("M.Kaihata", "12345").Signup(); err != nil {
		return errors.Wrap(err, "Signupに失敗しました")
	}
	if err = s.c.Login(); err != nil {
		return errors.Wrap(err, "Loginできません")
	}
	if err = s.c.AddScore(1); err != nil {
		return errors.Wrap(err, "Score追加ができません")
	}
	res, err = s.c.Info()
	if err != nil {
		return errors.Wrap(err, "/user にリクエスト失敗")
	}
	if res.User.UserID != "M.Kaihata" ||
		res.Score.BestScore != 1 ||
		res.Score.TotalScore != 1 ||
		res.Score.TotalRank == 47517 ||
		res.Score.BestRank == 47517 {
		return errors.Wrap(err, "/user のレスポンスが正しくありません")
	}
	err = s.c.ScoresCallback(ctx, "/latest_scores", 3, 1, func(s []Score, _ int) error {
		if len(s) != 3 || s[0].Name != "M.Kaihata" || s[0].Score != 1 {
			return errors.Errorf("/latest_scores が正しくありません")
		}
		for _, score := range s {
			if err := ScoreCheck(score); err != nil {
				return errors.Wrap(err, "/latest_scores が正しくありません")
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// 一般的にレベルアップしていくシナリオ
type PlayerScenario struct {
	*ScenarioBase
	count, page, loop int
}

func NewPlayerScenario(base string, factor float64, count, page, loop int) (*PlayerScenario, error) {
	bs, err := NewScenarioBase(base, 15*time.Second, factor)
	if err != nil {
		return nil, err
	}
	return &PlayerScenario{bs, count, page, loop}, nil
}

func (s *PlayerScenario) Run(ctx context.Context) error {
	for ; 0 < s.loop; s.loop-- {
		if err := s.c.RunStatics(ctx, TopPageStaticPaths); err != nil {
			return err
		}
		if err := s.c.RunPlayGames(ctx, rand.Intn(5)+1); err != nil {
			return err
		}
		if err := s.c.RunStatics(ctx, TopPageStaticPaths); err != nil {
			return err
		}
		if err := s.c.RunRankingScoresLoop(ctx, s.count, s.page); err != nil {
			return err
		}
		if err := s.c.RunLatestScoresLoop(ctx, s.count*2, s.page); err != nil {
			return err
		}
	}
	return nil
}

type ScrollScenario struct {
	*ScenarioBase
	count int
}

func NewScrollScenarioScenario(base string, factor float64, count int) (*ScrollScenario, error) {
	bs, err := NewScenarioBase(base, 10*time.Second, factor)
	if err != nil {
		return nil, err
	}
	return &ScrollScenario{bs, count}, nil
}

func (s *ScrollScenario) Run(ctx context.Context) error {
	if err := s.c.RunRankingScoresLoop(ctx, s.count, 10000); err != nil {
		return err
	}
	if err := s.c.RunLatestScoresLoop(ctx, s.count, 10000); err != nil {
		return err
	}
	return nil
}

func (s *ScrollScenario) Count() int {
	return s.count
}

type ScaleupScenario struct {
	*ScenarioBase
	count, page, loop int
}

func NewScaleupScenario(base string, factor float64, count, page, loop int) (*ScaleupScenario, error) {
	bs, err := NewScenarioBase(base, 10*time.Second, factor)
	if err != nil {
		return nil, err
	}
	return &ScaleupScenario{bs, count, page, loop}, nil
}

func (s *ScaleupScenario) Run(ctx context.Context) error {
	// for ; 0 < s.loop; s.loop-- {
	// 	if err := s.c.RunStatics(ctx, TopPageStaticPaths); err != nil {
	// 		return err
	// 	}
	// }
	if err := s.c.RunRankingScoresLoop(ctx, s.count, s.page); err != nil {
		return err
	}
	if err := s.c.RunLatestScoresLoop(ctx, s.count+100, s.page); err != nil {
		return err
	}
	return nil
}
