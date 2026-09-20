// Copyright (c) 2026 The btcd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaincfg

// The temporary long coinbase maturity rule, as deployed by Bitcoin Knots for the BLAKE2b
// chain. It raises the hundred block wait on newly mined coins to roughly forty five days.
//
// The rule has two faces and they do not agree, which is the whole reason this file exists
// rather than a single larger constant:
//
//   - Consensus rejects a spend only while the rule is active, that is only for a
//     transaction included between the enforce height and the release height, and only for
//     a coinbase mined at or after the start height. Everything else keeps the hundred
//     block rule. RequiredCoinbaseMaturity answers this.
//
//   - Relay is blunter. An upgraded node requires CoinbaseMaturityLong depth of every
//     coinbase spend it accepts into its mempool, whatever height the coinbase was mined
//     at and whatever the heights say, and it keeps doing so after the release height.
//     RelayCoinbaseMaturity answers this.
//
// So a coinbase mined well before the fork, with two hundred confirmations, is a valid
// spend that no upgraded node will relay. Anything choosing coins to spend wants the relay
// answer; only block validation wants the consensus one.

// CoinbaseMaturityLongDeployed reports whether the long coinbase maturity rule is
// configured for this network. A Params built without the fields, which is every Params
// written before the rule existed, reports false and behaves exactly as it did.
func (p *Params) CoinbaseMaturityLongDeployed() bool {
	return p.CoinbaseMaturityLong > 0 &&
		p.CoinbaseMaturityLongReleaseHeight > 0
}

// CoinbaseMaturityLongActiveAt reports whether consensus enforces the long rule against a
// transaction included at the given height.
func (p *Params) CoinbaseMaturityLongActiveAt(height int32) bool {
	if !p.CoinbaseMaturityLongDeployed() {
		return false
	}

	return height >= p.CoinbaseMaturityLongEnforceHeight &&
		height < p.CoinbaseMaturityLongReleaseHeight
}

// RequiredCoinbaseMaturity returns the depth a coinbase mined at coinbaseHeight must have
// reached before a transaction included at spendHeight may spend it without breaking
// consensus.
func (p *Params) RequiredCoinbaseMaturity(coinbaseHeight, spendHeight int32) int32 {
	if p.CoinbaseMaturityLongActiveAt(spendHeight) &&
		coinbaseHeight >= p.CoinbaseMaturityLongStartHeight {

		return p.CoinbaseMaturityLong
	}

	return int32(p.CoinbaseMaturity)
}

// RelayCoinbaseMaturity returns the depth a coinbase output must have reached before an
// upgraded node will accept a spend of it into its mempool.
//
// This is the number a wallet wants. It is at least as strict as consensus at every
// height, so a coin it clears cannot be refused by either.
func (p *Params) RelayCoinbaseMaturity() int32 {
	if !p.CoinbaseMaturityLongDeployed() {
		return int32(p.CoinbaseMaturity)
	}

	return p.CoinbaseMaturityLong
}
