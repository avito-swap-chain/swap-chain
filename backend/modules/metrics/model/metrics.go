package model

import (
	"errors"
	"time"
)

var ErrForbidden = errors.New("admin access is required for product metrics")

type RejectionReason struct {
	Reason string
	Count  int64
}

type Funnel struct {
	GeneratedAt                    time.Time
	EligibleItems                  int64
	ItemsWithChain                 int64
	ItemsWithChainRate             *float64
	AverageTimeToFirstChainSeconds *float64
	DecidedChains                  int64
	AcceptedChains                 int64
	AcceptanceRate                 *float64
	CompletedChains                int64
	DeliveryCompletionRate         *float64
	RejectionReasons               []RejectionReason
}
