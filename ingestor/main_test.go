package main

import (
	"testing"
	"math/big"
)

func TestBlockRangeRequest(t *testing.T) {
	r, err := getDeployments(
		"https://arb1.arbitrum.io/rpc",
		42161,
		new(big.Int).SetInt64(340010001),
		new(big.Int).SetInt64(340020901),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(r) == 0 {
		t.FailNow()
	}
}

func TestGetBlockHeights(t *testing.T) {
	for k, x := range getBlockHeights() {
		if x == nil {
			t.Fatalf("chain id block height: %v: %v", k, x)
		}
	}
}
