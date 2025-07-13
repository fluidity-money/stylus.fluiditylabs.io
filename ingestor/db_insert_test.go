package main

import "testing"

func TestTemplate(t *testing.T) {
	_, err := genInsert([]ContractDeployment{{}, {}}...)
	if err != nil {
		t.Fatalf("gen template: %v", err)
	}
}
