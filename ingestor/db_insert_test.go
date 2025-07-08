package main

import "testing"

func TestTemplate(t *testing.T) {
	x, err := genInsert([]ContractDeployment{{}, {}}...)
	if err != nil {
		t.Fatalf("gen template: %v", err)
	}
	t.Fatal(x)
}
