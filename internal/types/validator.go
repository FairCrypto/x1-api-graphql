// Package types implements different core types of the API.
package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"time"
)

// Validator represents a validator information.
type Validator struct {
	Id               hexutil.Big    `json:"id"`
	StakerAddress    common.Address `json:"address"`
	TotalStake       *hexutil.Big   `json:"totalStake"`
	Status           hexutil.Uint64 `json:"status"`
	CreatedEpoch     hexutil.Uint64 `json:"createdEpoch"`
	CreatedTime      hexutil.Uint64 `json:"createdTime"`
	DeactivatedEpoch hexutil.Uint64 `json:"deactivatedEpoch"`
	DeactivatedTime  hexutil.Uint64 `json:"deactivatedTime"`
}

// OfflineValidator represents a validator with current downtime.
type OfflineValidator struct {
	ID         uint64
	Address    common.Address
	DownTime   Downtime
	DownBlocks uint64
}

// ValidatorStatus represents a validator status information for the API.
type ValidatorStatus struct {
	Id               uint64    `json:"id"`
	Address          string    `json:"address"`
	TotalStaked      uint64    `json:"totalStaked"`
	Status           uint64    `json:"status"`
	CreatedEpoch     uint64    `json:"createdEpoch"`
	CreatedTime      time.Time `json:"createdTime"`
	DeactivatedEpoch uint64    `json:"deactivatedEpoch"`
	DeactivatedTime  time.Time `json:"deactivatedTime"`
	IsActive         bool      `json:"isActive"`
	IsWithdrawn      bool      `json:"isWithdrawn"`
	IsOffline        bool      `json:"isOffline"`
	IsCheater        bool      `json:"isCheater"`
	IsVoting         bool      `json:"isVoting"`
	DownTime         Dt        `json:"downTime"`
}

// Dt represents a downtime information.
type Dt struct {
	Time   uint64 `json:"time"`
	Blocks uint64 `json:"blocks"`
}

type Downtime uint64

func (d Downtime) String() string {
	return time.Duration(d).Round(time.Second).String()
}
