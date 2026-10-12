package blockchain

import (
	"math/big"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
)

// linearHeaderCtx is a HeaderCtx over a made-up chain: entry i is the block at
// height i.
type linearHeaderCtx struct {
	bits   []uint32
	times  []time.Time
	height int32
}

func (l linearHeaderCtx) Height() int32    { return l.height }
func (l linearHeaderCtx) Bits() uint32     { return l.bits[l.height] }
func (l linearHeaderCtx) Timestamp() int64 { return l.times[l.height].Unix() }
func (l linearHeaderCtx) Parent() HeaderCtx {
	return l.RelativeAncestorCtx(1)
}
func (l linearHeaderCtx) RelativeAncestorCtx(distance int32) HeaderCtx {
	if l.height-distance < 0 {
		return nil
	}
	return linearHeaderCtx{l.bits, l.times, l.height - distance}
}

// shiftedBits applies Knots' one-off shift independently of the code under
// test: the target of nBits times 2^shift, capped at the proof of work limit.
func shiftedBits(bits uint32, params *chaincfg.Params) uint32 {
	target := CompactToBig(bits)
	limit := params.PowLimit
	shift := uint(params.Blake2bTargetShift)
	if target.Cmp(new(big.Int).Rsh(limit, shift)) > 0 {
		return BigToCompact(limit)
	}
	return BigToCompact(new(big.Int).Lsh(target, shift))
}

// chainUpTo builds heights 0..last with the given bits, ten minutes apart.
func chainUpTo(last int32, bits func(int32) uint32,
	start time.Time) linearHeaderCtx {

	l := linearHeaderCtx{height: last}
	for h := int32(0); h <= last; h++ {
		l.bits = append(l.bits, bits(h))
		l.times = append(l.times, start.Add(time.Duration(h)*10*time.Minute))
	}
	return l
}

// TestBlake2bDifficultyTestNet4Activation covers testnet4's minimum difficulty
// rules at the activation block, as in Knots' GetNextWorkRequired: a block
// more than 20 minutes late gets the limit (Knots returns it before the shift,
// and shifting the limit would give the limit anyway), and the look-back to
// the last real difficulty is shifted.
func TestBlake2bDifficultyTestNet4Activation(t *testing.T) {
	params := chaincfg.TestNet4Params
	params.Blake2bHeight = 20
	c := mainnetChainCtx{&params}

	const realBits = 0x1900ffff
	start := time.Unix(1_700_000_000, 0)
	last := params.Blake2bHeight - 1

	// Every block at the real difficulty.
	chain := chainUpTo(last, func(int32) uint32 { return realBits }, start)
	onTime := chain.times[last].Add(10 * time.Minute)
	late := chain.times[last].Add(21 * time.Minute)

	got, err := calcNextRequiredDifficulty(chain, onTime, c)
	if err != nil {
		t.Fatal(err)
	}
	if want := shiftedBits(realBits, &params); got != want {
		t.Fatalf("on time: got %08x, want %08x (shifted)", got, want)
	}
	if got == realBits {
		t.Fatalf("on time: the shift was not applied")
	}

	got, err = calcNextRequiredDifficulty(chain, late, c)
	if err != nil {
		t.Fatal(err)
	}
	if got != params.PowLimitBits {
		t.Fatalf("late: got %08x, want the limit %08x unshifted",
			got, params.PowLimitBits)
	}

	// The last blocks before activation were mined at the minimum
	// difficulty: the look-back finds the real difficulty and shifts it.
	chain = chainUpTo(last, func(h int32) uint32 {
		if h > last-5 {
			return params.PowLimitBits
		}
		return realBits
	}, start)
	got, err = calcNextRequiredDifficulty(chain, onTime, c)
	if err != nil {
		t.Fatal(err)
	}
	if want := shiftedBits(realBits, &params); got != want {
		t.Fatalf("look-back: got %08x, want %08x", got, want)
	}

	// One block later the shift no longer applies.
	chain = chainUpTo(last+1, func(int32) uint32 { return realBits }, start)
	got, err = calcNextRequiredDifficulty(
		chain, chain.times[last+1].Add(10*time.Minute), c,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != realBits {
		t.Fatalf("after activation: got %08x, want %08x", got, realBits)
	}
}

// TestBlake2bDifficultyRegtestActivation covers a network without retargeting:
// Knots shifts its result too, and the shift caps at the limit, so the
// activation block keeps the limit. The genesis block has no parent and must
// not trip the activation check.
func TestBlake2bDifficultyRegtestActivation(t *testing.T) {
	params := chaincfg.RegressionNetParams
	params.Blake2bHeight = 150
	params.Blake2bTargetShift = 20
	c := mainnetChainCtx{&params}

	start := time.Unix(1_700_000_000, 0)
	chain := chainUpTo(params.Blake2bHeight-1,
		func(int32) uint32 { return params.PowLimitBits }, start)

	got, err := calcNextRequiredDifficulty(
		chain, chain.times[chain.height].Add(10*time.Minute), c,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != params.PowLimitBits {
		t.Fatalf("activation: got %08x, want the limit %08x",
			got, params.PowLimitBits)
	}
	if want := shiftedBits(params.PowLimitBits, &params); got != want {
		t.Fatalf("activation: got %08x, Knots' shift gives %08x", got, want)
	}

	got, err = calcNextRequiredDifficulty(nil, start, c)
	if err != nil {
		t.Fatal(err)
	}
	if got != params.PowLimitBits {
		t.Fatalf("genesis: got %08x, want %08x", got, params.PowLimitBits)
	}
}

// TestBlake2bDifficultyRetargetBoundary covers an activation height that
// falls on a retarget boundary (neither mainnet's nor testnet4's does): Knots
// shifts the freshly retargeted target, so the shift must apply on that path
// too.
func TestBlake2bDifficultyRetargetBoundary(t *testing.T) {
	params := chaincfg.MainNetParams
	params.Blake2bHeight = 2016
	c := mainnetChainCtx{&params}

	const realBits = 0x1900ffff
	start := time.Unix(1_700_000_000, 0)

	// Blocks every 9 minutes, so the retarget moves the target.
	chain := linearHeaderCtx{height: params.Blake2bHeight - 1}
	for h := int32(0); h < params.Blake2bHeight; h++ {
		chain.bits = append(chain.bits, realBits)
		chain.times = append(chain.times,
			start.Add(time.Duration(h)*9*time.Minute))
	}
	next := chain.times[chain.height].Add(10 * time.Minute)

	retargeted, shiftable, err := calcNextRequiredDifficultyBase(
		chain, next, c,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !shiftable || retargeted == realBits {
		t.Fatalf("expected a retarget away from %08x, got %08x "+
			"(shiftable %v)", realBits, retargeted, shiftable)
	}

	got, err := calcNextRequiredDifficulty(chain, next, c)
	if err != nil {
		t.Fatal(err)
	}
	if want := shiftedBits(retargeted, &params); got != want {
		t.Fatalf("got %08x, want the retarget %08x shifted: %08x",
			got, retargeted, want)
	}
}
