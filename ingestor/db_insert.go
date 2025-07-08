package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"math/big"
	"strconv"
	"text/template"
)

var tmplInsert = template.Must(template.New("").
	Funcs(template.FuncMap{
		"add": func(x, y int) int {
			return x + y
		},
		"mul": func(x, y int) int {
			return x * y
		},
	}).
	Parse(`
WITH checkpoint_update AS (
	SELECT stylus_update_ingestor_checkpoint_1($1, $2)
),
contract_inserts AS (
	SELECT stylus_insert_contract_1(
		cd.p1,
		cd.p2,
		cd.p3,
		cd.p4,
		cd.p5,
		cd.p6,
		cd.p7,
		cd.p8,
		cd.p9
	)
	FROM checkpoint_update cu
	CROSS JOIN (
		VALUES
			{{range $i, $x := . }}($1, ${{ mul $i 8 | add 3 }}, ${{ mul $i 8 | add 4 }}, ${{ mul $i 8 | add 5 }}, ${{ mul $i 8 | add 6 }}, ${{ mul $i 8 | add 7 }}, ${{ mul $i 8 | add 8 }}, ${{ mul $i 8 | add 9 }}, ${{ mul $i 8 | add 10 }}){{ if lt (add $i 1) (len $)}},{{end}}
			{{end}}
		) AS cd(p1, p2, p3, p4, p5, p6, p7, p8, p9)
	)
SELECT * from contract_inserts`,
	),
)

func genInsert(d ...ContractDeployment) (string, error) {
	var buf bytes.Buffer
	if err := tmplInsert.Execute(&buf, d); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// execDbInsert is a lazy TEXT-based insertion of values using the
// database, where coercion happens, done this way so we don't need to
// bother with local types (like in the main 9lives/Longtail repository).
func execDbInsert(db *sql.DB, chainId uint64, from *big.Int, deployments ...ContractDeployment) error {
	args := make([]any, 2, (len(deployments)*10)+2)
	args[0] = strconv.FormatUint(chainId, 10)
	args[1] = from.String()
	for _, d := range deployments {
		args = append(args,
			d.BlockNumber,
			d.BlockHash,
			d.TransactionHash,
			d.Address,
			d.CodeHash,
			d.ModuleHash,
			d.DataFee,
			d.Version,
		)
	}
	s, err := genInsert(deployments...)
	if err != nil {
		return fmt.Errorf("gen insert: %v", err)
	}
	if _, err := db.Exec(s, args...); err != nil {
		return fmt.Errorf("exec: %v", err)
	}
	return nil
}
