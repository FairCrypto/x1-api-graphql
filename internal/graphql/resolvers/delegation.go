// Package resolvers implements GraphQL resolvers to incoming API requests.
package resolvers

import (
	"fantom-api-graphql/internal/repository"
	"fantom-api-graphql/internal/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"golang.org/x/sync/singleflight"
	"math/big"
	"time"
)

// dlgWithdrawalsListDefaultLength is the default max len of withdrawals list.
const dlgWithdrawalsListDefaultLength = 25

// Delegator represents resolvable delegator detail.
type Delegation struct {
	types.Delegation
	cg *singleflight.Group
}

// NewDelegation creates new instance of resolvable Delegator.
func NewDelegation(d *types.Delegation) *Delegation {
	return &Delegation{Delegation: *d, cg: new(singleflight.Group)}
}

// Delegation resolves details of a delegator by it's address.
func (rs *rootResolver) Delegation(args *struct {
	Address common.Address
	Staker  hexutil.Big
}) (*Delegation, error) {
	// get the delegator detail from backend
	d, err := repository.R().Delegation(&args.Address, &args.Staker)
	if err != nil {
		return nil, err
	}

	return NewDelegation(d), nil
}

// Amount returns total delegated amount for the delegator.
func (del Delegation) Amount() (hexutil.Big, error) {
	// get the base amount delegated
	base, err := repository.R().DelegationAmountStaked(&del.Address, del.ToStakerId)
	if err != nil {
		return hexutil.Big{}, err
	}

	// get the sum of all pending withdrawals
	wd, err := del.pendingWithdrawalsValue()
	if err != nil {
		return hexutil.Big{}, err
	}
	val := new(big.Int).Add(base, wd)
	return (hexutil.Big)(*val), nil
}

// pendingWithdrawalsValue returns total amount of tokens
// locked in pending withdrawals for the delegation.
func (del Delegation) pendingWithdrawalsValue() (*big.Int, error) {
	// call for it only once
	val, err, _ := del.cg.Do("withdraw-total", func() (interface{}, error) {
		return repository.R().WithdrawRequestsPendingTotal(&del.Address, del.ToStakerId)
	})
	if err != nil {
		return nil, err
	}
	return val.(*big.Int), nil
}

// AmountInWithdraw returns total delegated amount in pending withdrawals for the delegator.
func (del Delegation) AmountInWithdraw() (hexutil.Big, error) {
	val, err := del.pendingWithdrawalsValue()
	if err != nil {
		return hexutil.Big{}, err
	}
	return (hexutil.Big)(*val), nil
}

// PendingRewards resolves pending rewards for the delegator account.
func (del Delegation) PendingRewards() (types.PendingRewards, error) {
	r, err := repository.R().PendingRewards(&del.Address, del.ToStakerId)
	if err != nil {
		return types.PendingRewards{}, err
	}
	return *r, nil
}

// ClaimedReward resolves the total amount of rewards received on the delegation.
func (del Delegation) ClaimedReward() (hexutil.Big, error) {
	return hexutil.Big{}, nil
}

// WithdrawRequests resolves partial withdraw requests of the delegator.
func (del Delegation) WithdrawRequests() ([]WithdrawRequest, error) {
	// pull list of withdrawals
	wr, err := repository.R().WithdrawRequests(&del.Address, del.ToStakerId, nil, dlgWithdrawalsListDefaultLength)
	if err != nil {
		return nil, err
	}

	// create withdrawals list from the collection
	list := make([]WithdrawRequest, len(wr.Collection))
	for i, req := range wr.Collection {
		list[i] = NewWithdrawRequest(req)
	}

	// return the final resolvable list
	return list, nil
}

// DelegationLock returns information about delegation lock
func (del Delegation) DelegationLock() (*types.DelegationLock, error) {
	// load the delegations lock only once
	dl, err, _ := del.cg.Do("lock", func() (interface{}, error) {
		return repository.R().DelegationLock(&del.Address, del.ToStakerId)
	})
	if err != nil {
		return nil, err
	}
	return dl.(*types.DelegationLock), nil
}

// IsDelegationLocked signals if the delegation is locked right now.
func (del Delegation) IsDelegationLocked() (bool, error) {
	lock, err := del.DelegationLock()
	if err != nil {
		return false, err
	}
	return lock != nil && uint64(lock.LockedUntil) < uint64(time.Now().UTC().Unix()), nil
}

// IsFluidStakingActive signals if the delegation is upgraded to Fluid Staking model.
func (del Delegation) IsFluidStakingActive() (bool, error) {
	return repository.R().DelegationFluidStakingActive(&del.Address, del.ToStakerId)
}

// LockedUntil resolves the end time of delegation.
func (del Delegation) LockedUntil() (hexutil.Uint64, error) {
	lock, err := del.DelegationLock()
	if err != nil {
		return hexutil.Uint64(0), err
	}
	return lock.LockedUntil, nil
}

// LockedFromEpoch resolves the epoch om which the lock has been created.
func (del Delegation) LockedFromEpoch() (hexutil.Uint64, error) {
	lock, err := del.DelegationLock()
	if err != nil {
		return hexutil.Uint64(0), err
	}
	return lock.LockedFromEpoch, nil
}

// OutstandingSFTM resolves the amount of outstanding sFTM tokens
// minted for this account.
func (del Delegation) OutstandingSFTM() (hexutil.Big, error) {
	val, err := repository.R().DelegationOutstandingSFTM(&del.Address, del.ToStakerId)
	if err != nil {
		return hexutil.Big{}, err
	}
	return *val, nil
}

// TokenizerAllowedToWithdraw resolves the tokenizer approval
// of the delegation withdrawal.
func (del Delegation) TokenizerAllowedToWithdraw() (bool, error) {
	// check the tokenizer lock status
	lock, err := repository.R().DelegationTokenizerUnlocked(&del.Address, del.ToStakerId)
	if err != nil {
		return false, err
	}
	return lock, nil
}
