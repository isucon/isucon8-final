package bench

import "github.com/Songmu/strrand"

type Random struct {
	passGen strrand.Generator
	nameGen strrand.Generator
	idGen   strrand.Generator
}

func NewRandom() (*Random, error) {
	passGen, err := strrand.New().CreateGenerator(`[abcdefghjkmnpqrstuvwxyz23456789]{12,16}`)
	if err != nil {
		return nil, err
	}
	nameGen, err = strrand.New().CreateGenerator(`[あ-んア-ンa-zA-Z0-9]{5,10}`)
	if err != nil {
		return nil, err
	}
	idGen, err := strrand.New().CreateGenerator(`[abcdefghjkmnpqrstuvwxyz23456789_-]{6,12}`)
	if err != nil {
		return nil, err
	}
	return &Random{
		passGen: passGen,
		nameGen: nameGen,
		idGen:   idGen,
	}, nil
}

func (b *Random) Password() string {
	return b.passGen.Generate()
}

func (b *Random) Name() string {
	return b.nameGen.Generate()
}

func (b *Random) ID() string {
	return b.idGen.Generate()
}
