/***************************************************************************
 *
 * Copyright (c) 2017 Baidu.com, Inc. All Rights Reserved
 * @author duanbing(duanbing@baidu.com)
 *
 **************************************************************************/

/**
 * @filename main.go
 * @desc
 * @create time 2018-04-19 15:49:26
**/
package main

import (
	"fmt"
	ec "github.com/duanbing/go-evm/core"
	"github.com/duanbing/go-evm/state"
	"github.com/duanbing/go-evm/types"
	"github.com/duanbing/go-evm/vm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"io/ioutil"
	"math/big"
	"os"
	"time"
)

var (
	testHash    = common.StringToHash("duanbing")
	fromAddress = common.StringToAddress("duanbing")
	toAddress   = common.StringToAddress("andone")
	amount      = big.NewInt(0)
	nonce       = uint64(0)
	gasLimit    = big.NewInt(100000)
	//coinbase    = common.HexToAddress("0x0000000000000000000000000000000000000000")
	coinbase = fromAddress
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func loadBin(filename string) []byte {
	code, err := ioutil.ReadFile(filename)
	must(err)
	return hexutil.MustDecode("0x" + string(code))
}
func loadAbi(filename string) abi.ABI {
	abiFile, err := os.Open(filename)
	must(err)
	defer abiFile.Close()
	abiObj, err := abi.JSON(abiFile)
	must(err)
	return abiObj
}

func main() {
	abiFileName := "./coin_sol_Coin.abi"
	binFileName := "./coin_sol_Coin.bin"
	data := loadBin(binFileName)

	msg := ec.NewMessage(fromAddress, &toAddress, nonce, amount, gasLimit, big.NewInt(0), data, false)
	cc := ChainContext{}
	ctx := ec.NewEVMContext(msg, cc.GetHeader(testHash, 0), cc, &fromAddress)
	dataPath := "/tmp/a.txt"
	os.Remove(dataPath)
	mdb, err := ethdb.NewLDBDatabase(dataPath, 100, 100)
	must(err)
	db := state.NewDatabase(mdb)

	root := common.Hash{}
	statedb, err := state.New(root, db)
	must(err)
	//set balance
	statedb.GetOrNewStateObject(fromAddress)
	statedb.GetOrNewStateObject(toAddress)
	statedb.AddBalance(fromAddress, big.NewInt(1e18))
	testBalance := statedb.GetBalance(fromAddress)
	fmt.Println("init testBalance =", testBalance)
	must(err)

	//	config := params.TestnetChainConfig
	config := params.MainnetChainConfig
	logConfig := vm.LogConfig{}
	structLogger := vm.NewStructLogger(&logConfig)
	vmConfig := vm.Config{Debug: true, Tracer: structLogger /*, JumpTable: vm.NewByzantiumInstructionSet()*/}

	evm := vm.NewEVM(ctx, statedb, config, vmConfig)
	contractRef := vm.AccountRef(fromAddress)
	fmt.Printf("%x\n", data)
	contractCode, contractAddr, gasLeftover, vmerr := evm.Create(contractRef, data, statedb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	must(vmerr)
	fmt.Printf("getcode:%x\n%x\n", contractCode, statedb.GetCode(contractAddr))

	statedb.SetBalance(fromAddress, big.NewInt(0).SetUint64(gasLeftover))
	testBalance = statedb.GetBalance(fromAddress)
	fmt.Println("after create contract, testBalance =", testBalance)
	abiObj := loadAbi(abiFileName)

	input, err := abiObj.Pack("minter")
	must(err)
	fmt.Println("11")
	evm.StateDB.SetCode(fromAddress, contractCode)
	outputs, gasLeftover, vmerr := evm.Call(contractRef, fromAddress, input, statedb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	fmt.Printf("minter is %x\n", outputs)
	fmt.Printf("call address %x\n", contractRef)
	must(vmerr)
	sender := outputs
	senderAcc := vm.AccountRef(common.BytesToAddress(sender))

	input, err = abiObj.Pack("mint", common.BytesToAddress(sender), big.NewInt(1000000))
	must(err)
	fmt.Println("22")
	outputs, gasLeftover, vmerr = evm.Call(senderAcc, fromAddress, input, statedb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	must(vmerr)

	statedb.SetBalance(fromAddress, big.NewInt(0).SetUint64(gasLeftover))
	testBalance = evm.StateDB.GetBalance(fromAddress)

	fmt.Println("33")
	input, err = abiObj.Pack("send", toAddress, big.NewInt(11))
	outputs, gasLeftover, vmerr = evm.Call(senderAcc, fromAddress, input, statedb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	must(vmerr)

	//send
	fmt.Println("44")
	input, err = abiObj.Pack("send", toAddress, big.NewInt(19))
	must(err)
	outputs, gasLeftover, vmerr = evm.Call(senderAcc, fromAddress, input, statedb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	must(vmerr)

	// get balance
	input, err = abiObj.Pack("balances", toAddress)
	must(err)
	outputs, gasLeftover, vmerr = evm.Call(contractRef, fromAddress, input, statedb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	must(vmerr)
	Print(outputs, "balances")

	// get balance
	input, err = abiObj.Pack("balances", common.BytesToAddress(sender))
	must(err)
	outputs, gasLeftover, vmerr = evm.Call(contractRef, fromAddress, input, statedb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	must(vmerr)
	Print(outputs, "balances")

	// get event
	logs := statedb.Logs()

	for _, log := range logs {
		fmt.Printf("%#v\n", log)
		for _, topic := range log.Topics {
			fmt.Printf("topic: %#v\n", topic)
		}
		fmt.Printf("data: %#v\n", log.Data)
	}

	idx := 1
	getstateFunc := func(key, value common.Hash) bool {
		fmt.Printf("------------- idx=%d, key=%v,value=%v\n", idx, key, value)
		idx += 1
		return true
	}
	statedb.ForEachStorage(contractAddr, getstateFunc)

	root, err = statedb.Commit(true)
	must(err)
	fmt.Println("Root Hash", root.Hex())

	err = db.TrieDB().Commit(root, true)
	must(err)

	mdb.Close()

	mdb2, err := ethdb.NewLDBDatabase("/tmp/test_state_storage", 100, 100)
	contractAddr = common.BytesToAddress([]byte("63e1d0fc342b9355bdda05adafa317edf644ddce"))
	must(err)
	db2 := state.NewDatabase(mdb2)
	//statedb2, err := state.New(common.HexToHash("0x0e5beb537865785f98e2dae33c2d88d70d086ad61569863e1bbd34801c7a54ef"), db2)
	statedb2, err := state.New(common.HexToHash("0x2c51c3995218789df648da9482a00668db162966523adbecd4643e44a974a746"), db2)
	must(err)
	testBalance = statedb2.GetBalance(fromAddress)
	fmt.Println("get testBalance =", testBalance)
	fmt.Printf("11111getcode:%x\n%x\n", contractCode, statedb2.GetCode(contractAddr))
}

func Print(outputs []byte, name string) {
	fmt.Printf("method=%s, output=%x\n", name, outputs)
}

type ChainContext struct{}

func (cc ChainContext) GetHeader(hash common.Hash, number uint64) *types.Header {

	return &types.Header{
		// ParentHash: common.Hash{},
		// UncleHash:  common.Hash{},
		Coinbase: fromAddress,
		//	Root:        common.Hash{},
		//	TxHash:      common.Hash{},
		//	ReceiptHash: common.Hash{},
		//	Bloom:      types.BytesToBloom([]byte("duanbing")),
		Difficulty: big.NewInt(1),
		Number:     big.NewInt(1),
		GasLimit:   1000000,
		GasUsed:    0,
		Time:       big.NewInt(time.Now().Unix()),
		Extra:      nil,
		//MixDigest:  testHash,
		//Nonce:      types.EncodeNonce(1),
	}
}
