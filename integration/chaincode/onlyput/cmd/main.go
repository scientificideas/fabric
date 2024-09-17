/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"fmt"
	"os"

	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric/integration/chaincode/onlyput"
)

func main() {
	err := shim.Start(&onlyput.OnlyPut{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Exiting SBE chaincode: %s", err)
		os.Exit(2)
	}
}
