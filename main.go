package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ledgerwatch/erigon-lib/common/hexutility"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/log/v3"
	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon/turbo/rpchelper"
	"github.com/ledgerwatch/erigon/rpc"
	"github.com/ledgerwatch/erigon/turbo/services"
	"github.com/ledgerwatch/erigon-lib/kv/kvcfg"
	"github.com/ledgerwatch/erigon/turbo/snapshotsync/freezeblocks"
	"github.com/ledgerwatch/erigon/core/rawdb/blockio"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/turbo/logging"
	"github.com/ledgerwatch/erigon/turbo/debug"
)

var (
	action     = flag.String("action", "", "action to execute")
	chaindata  = flag.String("chaindata", "chaindata", "path to the chaindata database file")
	output     = flag.String("output", "", "output path")
	account    = flag.String("account", "0x", "specifies account to investigate")
)

func main() {
	debug.RaiseFdLimit()
	flag.Parse()

	logging.SetupLogger("hack")


	var err error
	switch *action {
	case "extractFullAccount":
		err = extractFullAccount(*chaindata, libcommon.HexToAddress(*account))
	case "extractAccountsStorage":
		err = ExtractAccountsStorage(*chaindata, *output)
	}

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

type GenesisAccount struct {
	Code    *string            `json:"code,omitempty"`
	Nonce   string             `json:"nonce"`
	Balance string             `json:"balance"`
	Storage *map[string]string `json:"storage,omitempty"`
}

type Genesis struct {
	Nonce         string                    `json:"nonce"`
	Timestamp     string                    `json:"timestamp"`
	ExtraData     string                    `json:"extraData"`
	GasLimit      string                    `json:"gasLimit"`
	Difficulty    string                    `json:"difficulty"`
	MixHash       string                    `json:"mixHash"`
	Coinbase      string                    `json:"coinbase"`
	Alloc         map[string]GenesisAccount `json:"alloc"`
	Number        string                    `json:"number"`
	GasUsed       string                    `json:"gasUsed"`
	ParentHash    string                    `json:"parentHash"`
	BaseFeePerGas *string                   `json:"baseFeePerGas"`
	ExcessBlobGas *string                   `json:"excessBlobGas"`
	BlobGasUsed   *string                   `json:"blobGasUsed"`
}

func ExtractAccountsStorage(chaindata string, output string) error {
	db := mdbx.MustOpen(chaindata)
	defer db.Close()

	if output == "" {
		// use the chaindata path as a relative path for the datadir dump
		path := filepath.Dir(chaindata)
		output = path
	}
	
	var genesis Genesis
	// Read Lastest StateRoot
	ctx := context.Background()
	tx, err := db.BeginRo(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	latestBlockNum, err := rpchelper.GetLatestFinishedBlockNumber(tx)
	if err != nil {
		return err
	}
	fmt.Println("latestBlockNum: ", latestBlockNum)
	latestBlockNum, blockHash, _, err := rpchelper.GetBlockNumber_zkevm(rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(latestBlockNum)), tx, nil)
	if err != nil {
		return err
	}
	fmt.Println("latestBlockNum2: ", latestBlockNum)
	fmt.Println("blockHash: ", blockHash)
	blockReader, _ := blocksIO(db)
	header, err := blockReader.Header(ctx, tx, blockHash, latestBlockNum)
	if err != nil {
		return err
	}
	fmt.Println("LatestStateRoot: ", header.Root)

	var count uint64
	var keys []string

	if err := db.View(context.Background(), func(tx kv.Tx) error {
		return tx.ForEach(kv.PlainState, nil, func(k, v []byte) error {
			if len(k) == 20 {
				count++
				keys = append(keys, common.Bytes2Hex(k))
			}
			return nil
		})
	}); err != nil {
		return err
	}
	genesis.Alloc = make(map[string]GenesisAccount)
	fmt.Printf("count=%d\n", count)
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Printf("Address: %s\n", k)
		genesisAccount, err := extractFullAccountToStruct(chaindata, libcommon.HexToAddress(k), db)
		if err != nil {
			return err
		}
		genesis.Alloc[k] = genesisAccount
	}
	genesis.Nonce = "0x0"
  	genesis.Timestamp = "0x0"
  	genesis.ExtraData = "0x68697665636861696e"
  	genesis.GasLimit = "0x23f3e20"
  	genesis.Difficulty = "0x20000"
  	genesis.MixHash = "0x0000000000000000000000000000000000000000000000000000000000000000"
  	genesis.Coinbase = "0x0000000000000000000000000000000000000000"
	genesis.Number = "0x0"
	genesis.GasUsed = "0x0"
	genesis.ParentHash = "0x0000000000000000000000000000000000000000000000000000000000000000"
	genesisBytes, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return err
	}
	// create a file to stores the genesis
	fileName := "regenesis.json"
	return os.WriteFile(fileName, genesisBytes, 0644)
}

func extractFullAccountToStruct(chaindata string, account libcommon.Address, db kv.RwDB) (GenesisAccount, error) {
	var genesisAccount GenesisAccount
	tx, txErr := db.BeginRo(context.Background())
	if txErr != nil {
		return GenesisAccount{}, txErr
	}
	defer tx.Rollback()

	reader := state.NewPlainStateReader(tx)
	a, err := reader.ReadAccountData(account)
	if err != nil {
		return GenesisAccount{}, err
	} else if a == nil {
		return GenesisAccount{}, fmt.Errorf("acc not found")
	}
	fmt.Printf("CodeHash:%x\nIncarnation:%d\nNonce:%d\nBalance:%s\n", a.CodeHash, a.Incarnation, a.Nonce, a.Balance.String())
	genesisAccount.Balance = a.Balance.Hex()
	genesisAccount.Nonce = fmt.Sprintf("%x", a.Nonce)
	c, err := tx.Cursor(kv.PlainState)
	if err != nil {
		return GenesisAccount{}, err
	}
	defer c.Close()
	slots := make(map[string]string)
	for k, v, e := c.Seek(account.Bytes()); k != nil; k, v, e = c.Next() {
		if e != nil {
			return GenesisAccount{}, e
		}
		if !bytes.HasPrefix(k, account.Bytes()) {
			break
		}
		if len(k) == 20 && libcommon.BytesToAddress(k) == account {
			fmt.Printf("Ignoring first entry: %x => %x\n", k, v)
			continue
		}
		// fmt.Printf("StorageSlot: %x => %x\n", k, v)
		// Store slots
		if len(k) != 60 {
			return GenesisAccount{}, fmt.Errorf("error: invalid k length. Length: ", len(k))
		}
		StoragePosition := fmt.Sprintf("0x%x", k[28:])
		slots[StoragePosition] = fmt.Sprintf("0x%064x", v)
	}
	// fmt.Printf("|%06d|%6d|\n", 12, 345)

	if len(slots) != 0 {
		genesisAccount.Storage = &slots
	}
	cc, err := tx.Cursor(kv.PlainContractCode)
	if err != nil {
		return GenesisAccount{}, err
	}
	defer cc.Close()
	// fmt.Printf("code hashes\n")
	for k, v, e := cc.Seek(account.Bytes()); k != nil; k, v, e = c.Next() {
		if e != nil {
			return GenesisAccount{}, e
		}
		if !bytes.HasPrefix(k, account.Bytes()) {
			break
		}
		if libcommon.BytesToHash(v) != a.CodeHash {
			return GenesisAccount{}, fmt.Errorf("Code hash different. Found: %x, Expected: %s", v, a.CodeHash.String())
		}
		// fmt.Printf("%x => %x\n", k, v)
		// Find byteCode
		res, err := reader.ReadAccountCode(account, a.Incarnation, a.CodeHash)
		if err != nil {
			return GenesisAccount{}, err
		} else if res == nil {
			return GenesisAccount{}, fmt.Errorf("Bytecode not found for address: %s", account.String())
		}
		// fmt.Println("Bytecode: ", hexutility.Bytes(res).String())
		c := hexutility.Bytes(res).String()
		genesisAccount.Code = &c
	}
	return genesisAccount, nil
}

func extractFullAccount(chaindata string, account libcommon.Address) error {
	db := mdbx.MustOpen(chaindata)
	defer db.Close()

	tx, txErr := db.BeginRo(context.Background())
	if txErr != nil {
		return txErr
	}
	defer tx.Rollback()

	reader := state.NewPlainStateReader(tx)
	a, err := reader.ReadAccountData(account)
	if err != nil {
		return err
	} else if a == nil {
		return fmt.Errorf("acc not found")
	}
	fmt.Printf("CodeHash:%x\nIncarnation:%d\nNonce:%d\nBalance:%s\n", a.CodeHash, a.Incarnation, a.Nonce, a.Balance.String())

	c, err := tx.Cursor(kv.PlainState)
	if err != nil {
		return err
	}
	defer c.Close()
	for k, v, e := c.Seek(account.Bytes()); k != nil; k, v, e = c.Next() {
		if e != nil {
			return e
		}
		if !bytes.HasPrefix(k, account.Bytes()) {
			break
		}
		if len(k) == 20 && libcommon.BytesToAddress(k) == account {
			fmt.Printf("Ignoring first entry: %x => %x\n", k, v)
			continue
		}
		fmt.Printf("StorageSlot: %x => %x\n", k, v)
		// TODO Store slots
		if len(k) != 60 {
			return fmt.Errorf("error: invalid k length. Length: ", len(k))
		}
		fmt.Printf("0x%x\n", k[28:])
	}
	cc, err := tx.Cursor(kv.PlainContractCode)
	if err != nil {
		return err
	}
	defer cc.Close()
	fmt.Printf("code hashes\n")
	for k, v, e := cc.Seek(account.Bytes()); k != nil; k, v, e = c.Next() {
		if e != nil {
			return e
		}
		if !bytes.HasPrefix(k, account.Bytes()) {
			break
		}
		if libcommon.BytesToHash(v) != a.CodeHash {
			return fmt.Errorf("Code hash different. Found: %x, Expected: %s", v, a.CodeHash.String())
		}
		fmt.Printf("%x => %x\n", k, v)
		// Find byteCode
		res, err := reader.ReadAccountCode(account, a.Incarnation, a.CodeHash)
		if err != nil {
			return err
		} else if res == nil {
			return fmt.Errorf("Bytecode not found for address: %s", account.String())
		}
		fmt.Println("Bytecode: ", hexutility.Bytes(res).String())
	}
	return nil
}

func blocksIO(db kv.RoDB) (services.FullBlockReader, *blockio.BlockWriter) {
	var histV3 bool
	if err := db.View(context.Background(), func(tx kv.Tx) error {
		histV3, _ = kvcfg.HistoryV3.Enabled(tx)
		return nil
	}); err != nil {
		panic(err)
	}
	br := freezeblocks.NewBlockReader(freezeblocks.NewRoSnapshots(ethconfig.BlocksFreezing{Enabled: false}, "", 0, log.New()), nil /* BorSnapshots */)
	bw := blockio.NewBlockWriter(histV3)
	return br, bw
}