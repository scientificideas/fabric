/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package smartbft

type Logger interface {
	Warnf(template string, args ...interface{})
	Panicf(template string, args ...interface{})
}
