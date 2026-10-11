// Copyright (c) 2026 The btcd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaincfg

// BLAKE2b proof of work, as deployed by Bitcoin Knots for the BLAKE2b chain
// (v29.4.1+): from Blake2bHeight on, every block carries a 164-byte "header v2"
// whose block id is a BLAKE2b construction (see wire/blake2b.go), and the
// target of the first such block is shifted once by Blake2bTargetShift. The
// rest of proof of work is unchanged: the block id must not exceed the target,
// and the target follows the usual retarget rules.
//
// A Params built without these fields, which is every Params written before
// the rule existed, reports Blake2bPowDeployed false; the blockchain package
// then refuses v2 headers rather than check them against rules it was not
// given.

// Blake2bPowDeployed reports whether BLAKE2b proof of work is configured for
// this network.
func (p *Params) Blake2bPowDeployed() bool {
	return p.Blake2bHeight > 0
}

// IsBlake2bHeight reports whether a block at the given height must be a v2
// header with BLAKE2b proof of work (Knots' Consensus::Params::IsBlake2bHeight).
func (p *Params) IsBlake2bHeight(height int32) bool {
	return p.Blake2bPowDeployed() && height >= p.Blake2bHeight
}
