package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/authz/types"
)

var _ types.MsgServer = Keeper{}

// GrantAuthorization implements the MsgServer.GrantAuthorization method.
func (k Keeper) GrantAuthorization(goCtx context.Context, msg *types.MsgGrantAuthorization) (*types.MsgGrantAuthorizationResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	grantee, err := sdk.AccAddressFromBech32(msg.Grantee)
	if err != nil {
		return nil, err
	}
	granter, err := sdk.AccAddressFromBech32(msg.Granter)
	if err != nil {
		return nil, err
	}

	authorization := msg.GetGrantAuthorization()
	// If the granted service Msg doesn't exist, we throw an error.
	if k.router.Handler(authorization.MethodName()) == nil {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidType, "%s doesn't exist.", authorization.MethodName())
	}

	err = k.Grant(ctx, grantee, granter, authorization, msg.Expiration)
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventGrantAuthorization,
			sdk.NewAttribute(types.AttributeKeyGrantType, authorization.MethodName()),
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
			sdk.NewAttribute(types.AttributeKeyGranterAddress, msg.Granter),
			sdk.NewAttribute(types.AttributeKeyGranteeAddress, msg.Grantee),
		),
	)

	return &types.MsgGrantAuthorizationResponse{}, nil
}

// RevokeAuthorization implements the MsgServer.RevokeAuthorization method.
func (k Keeper) RevokeAuthorization(goCtx context.Context, msg *types.MsgRevokeAuthorization) (*types.MsgRevokeAuthorizationResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	grantee, err := sdk.AccAddressFromBech32(msg.Grantee)
	if err != nil {
		return nil, err
	}
	granter, err := sdk.AccAddressFromBech32(msg.Granter)
	if err != nil {
		return nil, err
	}

	err = k.Revoke(ctx, grantee, granter, msg.MethodName)
	if err != nil {
		return nil, err
	}
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventRevokeAuthorization,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
			sdk.NewAttribute(types.AttributeKeyGrantType, msg.MethodName),
			sdk.NewAttribute(types.AttributeKeyGranterAddress, msg.Granter),
			sdk.NewAttribute(types.AttributeKeyGranteeAddress, msg.Grantee),
		),
	)

	return &types.MsgRevokeAuthorizationResponse{}, nil
}

// ExecAuthorized implements the MsgServer.ExecAuthorized method.
func (k Keeper) ExecAuthorized(goCtx context.Context, msg *types.MsgExecAuthorized) (*types.MsgExecAuthorizedResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	grantee, err := sdk.AccAddressFromBech32(msg.Grantee)
	if err != nil {
		return nil, err
	}
	msgs, err := msg.GetServiceMsgs()
	if err != nil {
		return nil, err
	}
	result, err := k.DispatchActions(ctx, grantee, msgs)
	if err != nil {
		return nil, err
	}
	return &types.MsgExecAuthorizedResponse{Result: result}, nil
}