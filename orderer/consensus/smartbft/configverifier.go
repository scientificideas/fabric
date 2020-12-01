/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package smartbft

import (
	"errors"

	"github.com/hyperledger/fabric-protos-go/common"
)

type ConfigBlockValidator struct {
}

func (cbv *ConfigBlockValidator) ValidateConfig(envelope *common.Envelope) error {
	return errors.New("not implemented")
}