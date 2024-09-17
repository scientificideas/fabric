/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package onlyput

import (
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	pb "github.com/hyperledger/fabric-protos-go-apiv2/peer"
)

type OnlyPut struct{}

// Init initializes chaincode
// ===========================
func (t *OnlyPut) Init(_ shim.ChaincodeStubInterface) *pb.Response {
	return shim.Success(nil)
}

// Invoke - Our entry point for Invocations
// ========================================
func (t *OnlyPut) Invoke(stub shim.ChaincodeStubInterface) *pb.Response {
	function, args := stub.GetFunctionAndParameters()
	fmt.Println("invoke is running " + function)

	switch function {
	case "invoke":
		return t.put(stub, args)
	default:
		// error
		fmt.Println("invoke did not find func: " + function)
		return shim.Error("Received unknown function invocation")
	}
}

// both params should be marshalled json data and base64 encoded
func (t *OnlyPut) put(stub shim.ChaincodeStubInterface, args []string) *pb.Response {
	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}
	num, _ := strconv.Atoi(args[0])

	for i := range num {
		key := "key" + strconv.Itoa(i)
		err := stub.PutState(key, []byte(key))
		if err != nil {
			return shim.Error(err.Error())
		}
	}
	return shim.Success(nil)
}
