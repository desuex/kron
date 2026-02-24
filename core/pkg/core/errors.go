package core

import "errors"

var (
	ErrInvalidIdentity     = errors.New("invalid identity")
	ErrInvalidWindow       = errors.New("invalid window")
	ErrInvalidWindowMode   = errors.New("invalid window mode")
	ErrInvalidDistribution = errors.New("invalid distribution")
	ErrInvalidSeedStrategy = errors.New("invalid seed strategy")
	ErrInvalidTimezone     = errors.New("invalid timezone")
	ErrInvalidConstraint   = errors.New("invalid constraint")
)
