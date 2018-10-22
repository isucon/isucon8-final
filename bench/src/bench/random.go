package bench

import (
	"github.com/Songmu/strrand"
	"bench/randnameja"
)

type Random struct {
	passGen strrand.Generator
	idGen   strrand.Generator
}

func NewRandom() (*Random, error) {
	passGen, err := strrand.New().CreateGenerator(`[abcdefghjkmnpqrstuvwxyz23456789]{12,16}`)
	if err != nil {
		return nil, err
	}
	idGen, err := strrand.New().CreateGenerator(`[abcdefghjkmnpqrstuvwxyz23456789_-]{6,12}`)
	if err != nil {
		return nil, err
	}
	return &Random{
		passGen: passGen,
		idGen:   idGen,
	}, nil
}

func (b *Random) Password() string {
	return b.passGen.Generate()
}

func (b *Random) Name() string {
	return randnameja.Generate()
}

func (b *Random) ID() string {
	return b.idGen.Generate()
}
