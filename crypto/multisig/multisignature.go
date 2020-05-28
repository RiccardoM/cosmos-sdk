package multisig

import (
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/types"
	tx "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/tendermint/tendermint/crypto"
)

// AminoMultisignature is used to represent the signature object used in the multisigs.
// Sigs is a list of signatures, sorted by corresponding index.
type AminoMultisignature struct {
	BitArray *types.CompactBitArray
	Sigs     [][]byte
}

// NewMultisig returns a new MultiSignature
func NewMultisig(n int) *tx.MultiSignature {
	return &tx.MultiSignature{BitArray: types.NewCompactBitArray(n), Signatures: make([]tx.SignatureV2, n)}
}

// GetIndex returns the index of pk in keys. Returns -1 if not found
func getIndex(pk crypto.PubKey, keys []crypto.PubKey) int {
	for i := 0; i < len(keys); i++ {
		if pk.Equals(keys[i]) {
			return i
		}
	}
	return -1
}

// AddSignature adds a signature to the multisig, at the corresponding index.
// If the signature already exists, replace it.
func AddSignature(mSig *tx.MultiSignature, sig tx.SignatureV2, index int) {
	newSigIndex := mSig.BitArray.NumTrueBitsBefore(index)
	// Signature already exists, just replace the value there
	if mSig.BitArray.GetIndex(index) {
		mSig.Signatures[newSigIndex] = sig
		return
	}
	mSig.BitArray.SetIndex(index, true)
	// Optimization if the index is the greatest index
	if newSigIndex == len(mSig.Signatures) {
		mSig.Signatures = append(mSig.Signatures, sig)
		return
	}
	// Expand slice by one with a dummy element, move all elements after i
	// over by one, then place the new signature in that gap.
	mSig.Signatures = append(mSig.Signatures, &tx.SingleSignature{})
	copy(mSig.Signatures[newSigIndex+1:], mSig.Signatures[newSigIndex:])
	mSig.Signatures[newSigIndex] = sig
}

// AddSignatureFromPubKey adds a signature to the multisig, at the index in
// keys corresponding to the provided pubkey.
func AddSignatureFromPubKey(mSig *tx.MultiSignature, sig tx.SignatureV2, pubkey crypto.PubKey, keys []crypto.PubKey) error {
	index := getIndex(pubkey, keys)
	if index == -1 {
		keysStr := make([]string, len(keys))
		for i, k := range keys {
			keysStr[i] = fmt.Sprintf("%X", k.Bytes())
		}

		return fmt.Errorf("provided key %X doesn't exist in pubkeys: \n%s", pubkey.Bytes(), strings.Join(keysStr, "\n"))
	}

	AddSignature(mSig, sig, index)
	return nil
}

