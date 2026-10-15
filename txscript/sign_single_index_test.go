package txscript

import (
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// TestSignSigHashSingleWithoutOutput checks that no ECDSA signature is made
// with SIGHASH_SINGLE for an input that has no matching output, legacy,
// segwit v0 or unified, with or without ANYONECANPAY: it would commit to no
// output at all. Signing the input that has one, or with another hash type,
// still works.
func TestSignSigHashSingleWithoutOutput(t *testing.T) {
	t.Parallel()

	key, _ := btcec.PrivKeyFromBytes([]byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	})
	pkScript, err := NewScriptBuilder().AddOp(OP_0).
		AddData(make([]byte, 20)).Script()
	if err != nil {
		t.Fatal(err)
	}

	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{
		Hash: chainhash.Hash{1},
	}})
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{
		Hash: chainhash.Hash{2},
	}})
	tx.AddTxOut(wire.NewTxOut(1000, pkScript))

	fetcher := NewMultiPrevOutFetcher(map[wire.OutPoint]*wire.TxOut{
		tx.TxIn[0].PreviousOutPoint: wire.NewTxOut(2000, pkScript),
		tx.TxIn[1].PreviousOutPoint: wire.NewTxOut(2000, pkScript),
	})
	hashes := NewTxSigHashes(tx, fetcher)

	sign := func(idx int, ht SigHashType) (error, error) {
		_, legacy := RawTxInSignature(tx, idx, pkScript, ht, key)
		_, witness := RawTxInWitnessSignature(
			tx, hashes, idx, 2000, pkScript, ht, key,
		)

		return legacy, witness
	}

	for _, ht := range []SigHashType{
		SigHashSingle,
		SigHashSingle | SigHashAnyOneCanPay,
		SigHashSingle | SigHashUnified,
		SigHashSingle | SigHashUnified | SigHashAnyOneCanPay,
	} {
		legacy, witness := sign(1, ht)
		for name, err := range map[string]error{
			"legacy": legacy, "witness": witness,
		} {
			if !IsErrorCode(err, ErrInvalidSigHashSingleIndex) {
				t.Fatalf("%s 0x%x without a matching output: "+
					"got %v", name, ht, err)
			}
		}
	}

	// With a matching output, or another hash type: signed.
	for _, c := range []struct {
		idx int
		ht  SigHashType
	}{
		{0, SigHashSingle},
		{0, SigHashSingle | SigHashAnyOneCanPay},
		{1, SigHashAll},
		{1, SigHashNone},
	} {
		legacy, witness := sign(c.idx, c.ht)
		if legacy != nil || witness != nil {
			t.Fatalf("input %d 0x%x: %v, %v", c.idx, c.ht, legacy,
				witness)
		}
	}
}
