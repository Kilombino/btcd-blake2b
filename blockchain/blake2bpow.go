// Copyright (c) 2026 The btcd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
)

// BLAKE2b proof of work, mirroring Bitcoin Knots v29.4.2 (src/pow.cpp and
// src/validation.cpp) so a light client can check BLAKE2b-chain headers on its
// own, the way it checks SHA256d ones:
//
//   - CheckBlockHeader: a v2 header's reserved flag bits (0xc0) must be clear
//     (bad-flags-highbits); its block id must not exceed the target, the same
//     comparison as for a SHA256d id. See checkProofOfWork.
//   - ContextualCheckBlockHeader: below Blake2bHeight a header must be
//     classic (bad-version-sha256d); from it on it must be v2
//     (bad-version-blake2b) and commit to its own height (bad-header-height).
//     See checkHeaderV2Context.
//   - GetNextWorkRequired: the target of the block at Blake2bHeight is the
//     usual one shifted left once by Blake2bTargetShift, capped at the proof
//     of work limit. See applyBlake2bTargetShift.
//
// The headline committed in the coinbase of the activation block is a block
// rule, not a header one, and stays with full validation.

// header2ReservedFlags are the top two bits of a v2 header's flags, reserved
// for a future hard fork.
const header2ReservedFlags = 0xc0

// checkHeaderV2Context checks that a header carries the proof of work its
// height requires. A network without BLAKE2b proof of work configured accepts
// classic headers only, and refuses a v2 one rather than check it against
// rules it was not given.
func checkHeaderV2Context(header *wire.BlockHeader, height int32,
	params *chaincfg.Params) error {

	if !params.Blake2bPowDeployed() {
		if header.IsV2() {
			return ruleError(ErrUnsupportedProofOfWork, "BLAKE2b "+
				"(header v2) proof of work is not configured for "+
				"network "+params.Name)
		}
		return nil
	}

	if !header.IsV2() {
		if params.IsBlake2bHeight(height) {
			str := fmt.Sprintf("block at height %d must carry BLAKE2b "+
				"proof of work (header v2) from height %d", height,
				params.Blake2bHeight)
			return ruleError(ErrBadHeaderV2, str)
		}
		return nil
	}

	if !params.IsBlake2bHeight(height) {
		str := fmt.Sprintf("block at height %d must carry SHA256d proof of "+
			"work until height %d", height, params.Blake2bHeight)
		return ruleError(ErrBadHeaderV2, str)
	}
	if header.Height != height {
		str := fmt.Sprintf("header v2 commits to height %d at height %d",
			header.Height, height)
		return ruleError(ErrBadHeaderV2, str)
	}
	return nil
}

// applyBlake2bTargetShift is Knots' ApplyBlake2bTargetShift: the target of the
// first block mined under BLAKE2b, from the one the usual rules give.
func applyBlake2bTargetShift(bits uint32, params *chaincfg.Params) uint32 {
	target := CompactToBig(bits)
	shift := params.Blake2bTargetShift
	if target.Cmp(new(big.Int).Rsh(params.PowLimit, shift)) > 0 {
		target = new(big.Int).Set(params.PowLimit)
	} else {
		target.Lsh(target, shift)
	}
	return BigToCompact(target)
}
