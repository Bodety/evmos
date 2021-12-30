package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/tharsis/evmos/x/incentives/types"
)

// RegisterIncentive creates an incentive for a contract
func (k Keeper) RegisterIncentive(
	ctx sdk.Context,
	contract common.Address,
	allocations sdk.DecCoins,
	epochs uint32,
) (*types.Incentive, error) {

	// check if the Incentives are globally enabled
	params := k.GetParams(ctx)
	if !params.EnableIncentives {
		return nil, sdkerrors.Wrap(
			types.ErrInternalIncentive,
			"incentives are currently disabled by governance",
		)
	}

	// check if the incentive is already registered
	if k.IsIncentiveRegistered(ctx, contract) {
		return nil, sdkerrors.Wrapf(
			types.ErrInternalIncentive,
			"incentive already registered: %s", contract,
		)
	}

	// check if the balance is > 0 for coins other than the mint denomination
	mintDenom := k.mintKeeper.GetParams(ctx).MintDenom
	for _, al := range allocations {
		if al.Denom != mintDenom {
			if !k.bankKeeper.HasSupply(ctx, al.Denom) {
				return nil, sdkerrors.Wrapf(
					sdkerrors.ErrInvalidCoins,
					"base denomination '%s' cannot have a supply of 0", al.Denom,
				)
			}
		}

		// check if each allocation is below the allocation limit
		if al.Amount.GT(params.AllocationLimit) {
			return nil, sdkerrors.Wrapf(
				types.ErrInternalIncentive,
				"allocation for denom '%s' (%v) cannot be above allocation limmit '%v' - ", al.Denom, al.Amount, params.AllocationLimit,
			)
		}
	}

	// check if the sum of all allocations for each denom (current + proposed) is
	// < 100%
	incentives := k.GetAllIncentives(ctx)
	if len(incentives) != 0 {
		currentAllocations := make(map[string]sdk.Dec)

		k.IterateIncentives(
			ctx,
			func(incentive types.Incentive) (stop bool) {
				for _, al := range incentive.Allocations {
					if al.Amount.Size() == 0 {
						continue
					}
					if _, ok := currentAllocations[al.Denom]; ok {
						currentAllocations[al.Denom] = currentAllocations[al.Denom].Add(al.Amount)
					} else {
						currentAllocations[al.Denom] = al.Amount
					}
				}
				return false
			},
		)

		for _, al := range allocations {
			// skip if no current allocations exist
			if _, ok := currentAllocations[al.Denom]; !ok {
				continue
			}

			allocationSum := al.Amount.Add(currentAllocations[al.Denom])
			if allocationSum.Size() > 1 {
				return nil, sdkerrors.Wrapf(
					types.ErrInternalIncentive,
					"Allocation for denom %s is lager than 100 percent: %v",
					al.Denom, allocationSum,
				)
			}
		}
	}

	// create incentive and set to store
	incentive := types.NewIncentive(contract, allocations, epochs)
	k.SetIncentive(ctx, incentive)

	return &incentive, nil
}

// RegisterIncentive deletes the incentive for a contract
func (k Keeper) CancelIncentive(
	ctx sdk.Context,
	contract common.Address,
) error {
	incentive, found := k.GetIncentive(ctx, contract)
	if !found {
		return sdkerrors.Wrapf(
			sdkerrors.ErrInvalidAddress,
			"unmatching contract '%s' ", contract,
		)
	}

	k.DeleteIncentive(ctx, incentive)
	return nil
}
