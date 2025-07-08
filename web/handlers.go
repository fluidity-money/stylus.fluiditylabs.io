package main

import (
	"bytes"
	"database/sql"
	_ "embed"
	"encoding/csv"
	"io"
	"log/slog"
	"net/http"
	"text/template"
	"time"
)

var (
	//go:embed index.html
	Index []byte
	//go:embed table.tmpl
	Table string
	//go:embed footer.html
	Footer []byte
)

// TmplTable to execute the table rendering on request.
var TmplTable = template.Must(template.New("table").Parse(Table))

// Result that's simply sent to the frontend, allowing the user to decide
// what they want to do with it.
type Result struct {
	Id                                                     int
	CreatedAt                                              time.Time
	ChainId, BlockNumber, TransactionHash, ContractAddress string
	ModuleHash, DataFee, CodeHash, BlockHash               string
	Version                                                int
}

func HandleHtmlView(db *sql.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(w, bytes.NewReader(Index)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			slog.Error("Error writing preamble", "err", err)
			return
		}
		rows, err := db.Query("SELECT * FROM stylus_contracts_unique_1")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			slog.Error("Failed to select unique contracts",
				"err", err,
			)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var res Result
			err = rows.Scan(
				&res.Id,
				&res.CreatedAt,
				&res.ChainId,
				&res.BlockNumber,
				&res.BlockHash,
				&res.TransactionHash,
				&res.ContractAddress,
				&res.CodeHash,
				&res.ModuleHash,
				&res.DataFee,
				&res.Version,
			)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				slog.Error("Failed to scan results",
					"err", err,
				)
				return
			}
			if err := TmplTable.Execute(w, res); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				slog.Error("Failed to execute template",
					"err", err,
				)
				return
			}
		}
		if _, err := io.Copy(w, bytes.NewReader(Footer)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			slog.Error("Error writing footer", "err", err)
			return
		}
	}
}

func HandleTsvView(db *sql.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Don't even bother with buffering here since the list isn't long.
		rows, err := db.Query("SELECT * FROM stylus_contracts_unique_1")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			slog.Error("Failed to select unique contracts",
				"err", err,
			)
			return
		}
		defer rows.Close()
		e := csv.NewWriter(w)
		e.Comma = '	'
		defer e.Flush()
		err = e.Write([]string{"id", "inserted at", "chain id", "block number", "block hash", "transaction hash", "contract address", "code hash", "module hash", "data fee", "version"})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			slog.Error("header write", "err", err)
			return
		}
		for rows.Next() {
			var c [11]string
			err = rows.Scan(
				&c[0],
				&c[1],
				&c[2],
				&c[3],
				&c[4],
				&c[5],
				&c[6],
				&c[7],
				&c[8],
				&c[9],
				&c[10],
			)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				slog.Error("Failed to scan contracts",
					"err", err,
				)
				return
			}
			if err := e.Write(c[:]); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				slog.Error("Failed send row",
					"err", err,
				)
				return
			}
		}
		slog.Info("done reading")
	}
}
