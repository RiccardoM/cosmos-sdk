package multisig

import (
	types "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/tendermint/tendermint/crypto"
)

type GetSignBytesFunc func(mode types.SignMode) ([]byte, error)

type MultisigPubKey interface {
	crypto.PubKey

	VerifyMultisignature(getSignBytes GetSignBytesFunc, sig *types.MultiSignature) bool
	GetPubKeys() []crypto.PubKey
}
