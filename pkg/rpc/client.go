package rpc

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient/gethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/taikoxyz/taiko-client/bindings"
)

const (
	defaultTimeout = 1 * time.Minute
)

// Client contains all L1/L2 RPC clients that a driver needs.
type Client struct {
	// Geth ethclient clients
	L1           *EthClient
	L2           *EthClient
	Apus *EthClient
	L2CheckPoint *EthClient
	// Geth gethclient clients
	L1GethClient *gethclient.Client
	L2GethClient *gethclient.Client
	ApusGethClient *gethclient.Client
	// Geth raw RPC clients
	L1RawRPC *rpc.Client
	L2RawRPC *rpc.Client
	ApusRawRPC *rpc.Client
	// Geth Engine API clients
	L2Engine *EngineClient
	// Protocol contracts clients
	TaikoL1    *bindings.TaikoL1Client
	TaikoL2    *bindings.TaikoL2Client
	TaikoToken *bindings.TaikoToken
	ApusTask *bindings.ApusTask
	ApusMarket *bindings.ApusMarket
	// Chain IDs
	L1ChainID *big.Int
	L2ChainID *big.Int
	ApusChainID *big.Int
}

// ClientConfig contains all configs which will be used to initializing an
// RPC client. If not providing L2EngineEndpoint or JwtSecret, then the L2Engine client
// won't be initialized.
type ClientConfig struct {
	L1Endpoint        string
	L2Endpoint        string
	L2CheckPoint      string
	TaikoL1Address    common.Address
	TaikoL2Address    common.Address
	TaikoTokenAddress common.Address
	L2EngineEndpoint  string
	JwtSecret         string
	RetryInterval     time.Duration
	Timeout           *time.Duration
	BackOffMaxRetrys  *big.Int
}

// NewClient initializes all RPC clients used by Taiko client softwares.
func NewClient(ctx context.Context, cfg *ClientConfig) (*Client, error) {
	//var apusRPCEndpoint = "http://1.117.58.173:85450x0eaD84bE2483bCFA7361D541037Aab3769174c41"
	//var apusMarketAddress = common.HexToAddress("0xB2e47AC772F07b82965005Daad790BB471DC81A6")
	var apusRPCEndpoint = "https://rpc.jolnir.taiko.xyz"
	var apusMarketAddress = common.HexToAddress("0xD1bA4979ca39154E79D2Fa7676AACc7E6367DB24")
	var apusTasktAddress = common.HexToAddress("0x8769e981120a2305d8a126c43762c0E3ac0843f0")
	ctxWithTimeout, cancel := ctxWithTimeoutOrDefault(ctx, defaultTimeout)
	defer cancel()

	if cfg.BackOffMaxRetrys == nil {
		defaultRetrys := new(big.Int).SetInt64(10)
		cfg.BackOffMaxRetrys = defaultRetrys
	}

	l1EthClient, err := DialClientWithBackoff(ctxWithTimeout, cfg.L1Endpoint, cfg.RetryInterval, cfg.BackOffMaxRetrys)
	if err != nil {
		return nil, err
	}

	l2EthClient, err := DialClientWithBackoff(ctxWithTimeout, cfg.L2Endpoint, cfg.RetryInterval, cfg.BackOffMaxRetrys)
	if err != nil {
		return nil, err
	}

	apusClient, err := DialClientWithBackoff(ctxWithTimeout, apusRPCEndpoint, cfg.RetryInterval, cfg.BackOffMaxRetrys)
	if err != nil {
		return nil, err
	}

	var (
		l1RPC *EthClient
		l2RPC *EthClient
		apusRPC *EthClient
	)
	if cfg.Timeout != nil {
		l1RPC = NewEthClientWithTimeout(l1EthClient, *cfg.Timeout)
		l2RPC = NewEthClientWithTimeout(l2EthClient, *cfg.Timeout)
		apusRPC = NewEthClientWithTimeout(apusClient, *cfg.Timeout)
	} else {
		l1RPC = NewEthClientWithDefaultTimeout(l1EthClient)
		l2RPC = NewEthClientWithDefaultTimeout(l2EthClient)
		apusRPC = NewEthClientWithDefaultTimeout(apusClient)
	}

	taikoL1, err := bindings.NewTaikoL1Client(cfg.TaikoL1Address, l1RPC)
	if err != nil {
		return nil, err
	}

	taikoL2, err := bindings.NewTaikoL2Client(cfg.TaikoL2Address, l2RPC)
	if err != nil {
		return nil, err
	}
	apusTask, err := bindings.NewApusTask(apusTasktAddress, apusRPC)
	if err != nil {
		return nil, err
	}

	apusMarket, err := bindings.NewApusMarket(apusMarketAddress, apusRPC)
	if err != nil {
		return nil, err
	}

	var taikoToken *bindings.TaikoToken
	if cfg.TaikoTokenAddress.Hex() != ZeroAddress.Hex() {
		taikoToken, err = bindings.NewTaikoToken(cfg.TaikoTokenAddress, l1RPC)
		if err != nil {
			return nil, err
		}
	}

	stateVars, err := taikoL1.GetStateVariables(&bind.CallOpts{Context: ctxWithTimeout})
	if err != nil {
		return nil, err
	}

	isArchive, err := IsArchiveNode(ctxWithTimeout, l1RPC, stateVars.GenesisHeight)
	if err != nil {
		return nil, err
	}

	if !isArchive {
		return nil, fmt.Errorf("error with RPC endpoint: node (%s) must be archive node", cfg.L1Endpoint)
	}

	l1RawRPC, err := rpc.Dial(cfg.L1Endpoint)
	if err != nil {
		return nil, err
	}

	l2RawRPC, err := rpc.Dial(cfg.L2Endpoint)
	if err != nil {
		return nil, err
	}

	apusRawRPC, err := rpc.Dial(apusRPCEndpoint)
	if err != nil {
		return nil, err
	}

	l1ChainID, err := l1RPC.ChainID(ctxWithTimeout)
	if err != nil {
		return nil, err
	}

	l2ChainID, err := l2RPC.ChainID(ctxWithTimeout)
	if err != nil {
		return nil, err
	}

	apusChainID, err := apusRPC.ChainID(ctxWithTimeout)
	if err != nil {
		return nil, err
	}

	// If not providing L2EngineEndpoint or JwtSecret, then the L2Engine client
	// won't be initialized.
	var l2AuthRPC *EngineClient
	if len(cfg.L2EngineEndpoint) != 0 && len(cfg.JwtSecret) != 0 {
		if l2AuthRPC, err = DialEngineClientWithBackoff(
			ctxWithTimeout,
			cfg.L2EngineEndpoint,
			cfg.JwtSecret,
			cfg.RetryInterval,
			cfg.BackOffMaxRetrys,
		); err != nil {
			return nil, err
		}
	}

	var l2CheckPoint *EthClient
	if len(cfg.L2CheckPoint) != 0 {
		l2CheckPointEthClient, err := DialClientWithBackoff(
			ctxWithTimeout,
			cfg.L2CheckPoint,
			cfg.RetryInterval,
			cfg.BackOffMaxRetrys)
		if err != nil {
			return nil, err
		}

		if cfg.Timeout != nil {
			l2CheckPoint = NewEthClientWithTimeout(l2CheckPointEthClient, *cfg.Timeout)
		} else {
			l2CheckPoint = NewEthClientWithDefaultTimeout(l2CheckPointEthClient)
		}
	}

	client := &Client{
		L1:           l1RPC,
		L2:           l2RPC,
		Apus: apusRPC,
		L2CheckPoint: l2CheckPoint,
		L1RawRPC:     l1RawRPC,
		L2RawRPC:     l2RawRPC,
		ApusRawRPC:   apusRawRPC,
		L1GethClient: gethclient.New(l1RawRPC),
		L2GethClient: gethclient.New(l2RawRPC),
		ApusGethClient: gethclient.New(apusRawRPC),
		L2Engine:     l2AuthRPC,
		TaikoL1:      taikoL1,
		TaikoL2:      taikoL2,
		TaikoToken:   taikoToken,
		ApusTask: 	  apusTask,
		ApusMarket:   apusMarket,
		L1ChainID:    l1ChainID,
		L2ChainID:    l2ChainID,
		ApusChainID: apusChainID,
	}

	if err := client.ensureGenesisMatched(ctxWithTimeout); err != nil {
		return nil, err
	}

	return client, nil
}
