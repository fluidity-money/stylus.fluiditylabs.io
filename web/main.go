// web: Serves a template rendered simple webapp that's generated as a
// result of the request to the Postgres database given. Sends everything
// it has, assuming that the edge will cache the requests here.

package main

import (
	"database/sql"
	"log"
	"log/slog"
	"net/http"
	"os"

	_ "github.com/lib/pq"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
)

const (
	EnvDatabaseUri = "SPN_DATABASE_URI"
	EnvBackendType = "SPN_LISTEN_BACKEND"
	EnvListenAddr  = "SPN_LISTEN_ADDR"
	EnvDebug       = "SPN_DEBUG"
)

func main() {
	logLevel := slog.LevelInfo
	if os.Getenv(EnvDebug) != "" {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})))
	db, err := sql.Open("postgres", os.Getenv(EnvDatabaseUri))
	if err != nil {
		log.Fatalf("database open: %v", err)
	}
	http.HandleFunc("/", HandleHtmlView(db))
	http.HandleFunc("/contracts.tsv", HandleTsvView(db))
	switch typ := os.Getenv(EnvBackendType); typ {
	case "lambda":
		lambda.Start(httpadapter.NewV2(http.DefaultServeMux).ProxyWithContext)
	case "http":
		err := http.ListenAndServe(os.Getenv(EnvListenAddr), nil)
		log.Fatalf(
			"err listening, %#v not set?: %v",
			EnvListenAddr,
			err,
		)
	default:
		log.Fatalf(
			"unexpected listen type: %#v, use either (lambda|http) for SPN_LISTEN_BACKEND",
			typ,
		)
	}
}
