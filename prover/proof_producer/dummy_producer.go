package producer

import (
	"bytes"
	"context"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/taikoxyz/taiko-client/bindings"
	"github.com/taikoxyz/taiko-client/bindings/encoding"
)

// DummyProofProducer always returns a dummy proof.
type DummyProofProducer struct {
	RandomDummyProofDelayLowerBound *time.Duration
	RandomDummyProofDelayUpperBound *time.Duration
	OracleProofSubmissionDelay      time.Duration
	OracleProverAddress             common.Address
	ProofWindow                     uint16
}

// RequestProof implements the ProofProducer interface.
func (d *DummyProofProducer) RequestProof(
	ctx context.Context,
	opts *ProofRequestOptions,
	blockID *big.Int,
	meta *bindings.TaikoDataBlockMetadata,
	header *types.Header,
	resultCh chan *ProofWithHeader,
) error {
	log.Info(
		"Request dummy proof",
		"blockID", blockID,
		"proposer", meta.Proposer,
		"height", header.Number,
		"hash", header.Hash(),
	)

	if opts.AssignedProver != encoding.OracleProverAddress && d.OracleProofSubmissionDelay != 0 {
		var delay time.Duration
		if time.Now().Unix()-int64(header.Time+uint64(d.ProofWindow)) >= int64(d.OracleProofSubmissionDelay.Seconds()) {
			delay = 0
		} else {
			delay = time.Duration(
				int64(d.OracleProofSubmissionDelay.Seconds())-(time.Now().Unix()-int64(header.Time+uint64(d.ProofWindow))),
			) * time.Second
		}

		log.Info(
			"Oracle proof submission delay",
			"blockID", blockID,
			"proposer", meta.Proposer,
			"assignedProver", opts.AssignedProver,
			"delay", delay,
		)

		time.AfterFunc(delay, func() {
			resultCh <- &ProofWithHeader{
				BlockID: blockID,
				Meta:    meta,
				Header:  header,
				ZkProof: bytes.Repeat([]byte{0xff}, 100),
				Degree:  CircuitsIdx,
				Opts:    opts,
			}
		})

		return nil
	}

	time.AfterFunc(d.proofDelay(), func() {
		resultCh <- &ProofWithHeader{
			BlockID: blockID,
			Meta:    meta,
			Header:  header,
			ZkProof: bytes.Repeat([]byte{0xff}, 100),
			Degree:  CircuitsIdx,
			Opts:    opts,
		}
	})

	return nil
}

// proofDelay calculates a random proof delay between the bounds.
func (d *DummyProofProducer) proofDelay() time.Duration {
	if d.RandomDummyProofDelayLowerBound == nil ||
		d.RandomDummyProofDelayUpperBound == nil ||
		*d.RandomDummyProofDelayUpperBound == time.Duration(0) {
		return time.Duration(0)
	}

	lowerSeconds := int(d.RandomDummyProofDelayLowerBound.Seconds())
	upperSeconds := int(d.RandomDummyProofDelayUpperBound.Seconds())

	randomDurationSeconds := rand.Intn((upperSeconds - lowerSeconds)) + lowerSeconds
	delay := time.Duration(randomDurationSeconds) * time.Second

	log.Info("Random dummy proof delay", "delay", delay)

	return delay
}

// Cancel cancels an existing proof generation.
func (d *DummyProofProducer) Cancel(ctx context.Context, blockID *big.Int) error {
	return nil
}
