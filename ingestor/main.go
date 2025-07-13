// ingestor: Request from several chains as defined in the chains.tsv
// file for events as emitted by the ArbOS activateProgram precompile.
// Store the results in the database. Runs, then exits. Might take some
// time to paginate safely using each RPC, but it's safe to be run on a
// spot instance or something that exits frequently, as long as it can be
// restarted without issue later. Insertions will take place in an atomic
// fashion using a CTE.

package main

import (
	"bytes"
	"database/sql"
	_ "embed"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"

	_ "github.com/lib/pq"

	ethAbi "github.com/ethereum/go-ethereum/accounts/abi"
	ethCommon "github.com/ethereum/go-ethereum/common"
)

// EnvDatabaseUri to look up and to return results for.
const EnvDatabaseUri = "SPN_DATABASE_URI"

// AddrPrecompile to use when making requests to the RPC to get the list
// of the activated programs.
const AddrPrecompile = `0x0000000000000000000000000000000000000071`

// PaginationAmount to use to send each lookup with an extra 5000 blocks.
var PaginationAmount = new(big.Int).SetInt64(5000)

//go:embed abi.json
var abiB []byte

//go:embed chains.tsv
var chainsB []byte

// Chain that was configured in chains.tsv.
type Chain struct {
	Name, Website, Rpc string
	From               *big.Int
	ChainId            uint64
}

var Chains []Chain

var abi, _ = ethAbi.JSON(bytes.NewReader(abiB))

type (
	ContractDeployment struct {
		ChainId                                        uint64
		BlockNumber, DataFee                           string
		CodeHash                                       string
		Address, Deployer, ModuleHash, TransactionHash string
		BlockHash                                      string
		Codesize                                       int64
		Version                                        int
	}

	request struct {
		chainId uint64
		rpc     string
		// from, until are remade *big.Int entirely for the life of the message.
		from, until *big.Int
	}

	requestErr struct {
		chainId uint64
		err     error
	}
)

type blockHeightResp struct {
	chainId uint64
	cur     *big.Int
}

func main() {
	// Start to look up the block ranges from the amount that we're currently
	// at to either the max of the current block height when the program
	// started, or the current amount plus the search range. One goroutine
	// will make requests to paginate, while a worker pool will make requests
	// to get results while simultaneously using the database to bump the
	// current range and make inserts, and update the main routine.
	// This means the worker can die inexplicably, though not tolerating more
	// than one worker.
	db, err := sql.Open("postgres", os.Getenv(EnvDatabaseUri))
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("db select: %v", err)
	}
	var (
		// Requests to look up the range.
		chanRequests = make(chan request)
		// Reported successes of lookup per chain id. The main routine knows
		// the current lookup limit. This approach was fine with a smaller number
		// of chains, but since we added more, it's become a bit messy and
		// the initial seeding of the chains should probably be rolled into the same
		// loop that does the sending. But a quick workaround is just to have a higher
		// buffer here for the initial tension with the success loop, previously 2 was
		// sufficient.
		chanSuccesses = make(chan uint64, 100)
	)
	for i := 0; i < runtime.NumCPU()*5; i++ {
		go func() {
			for r := range chanRequests {
				deployments, err := getDeployments(r.rpc, r.chainId, r.from, r.until)
				if err != nil {
					log.Fatalf("lookup: %v: %v", r.chainId, err)
				}
				if len(deployments) > 0 {
					if err := execDbInsert(db, r.chainId, r.until, deployments...); err != nil {
						// Database error: kill the program immediately.
						log.Fatalf("insert db: %v", err)
					}
				} else {
					_,  err := db.Exec(
						"SELECT stylus_update_ingestor_checkpoint_1($1, $2)",
						r.chainId,
						r.until.String(),
					)
					if err != nil {
						log.Fatalf("checkpoint insert: %v", err)
					}
				}
				chanSuccesses <- r.chainId
			}
		}()
	}
	rpcs := make(map[uint64]string, len(Chains))
	for _, c := range Chains {
		rpcs[c.ChainId] = c.Rpc
	}
	blockHeights := getBlockHeights()
	// From blocks are edited with the current position in the pagination.
	fromBlocks := getFromBlocks(db)
	go func() {
		// Seed everything using a separate routine.
		for _, c := range Chains {
			from := new(big.Int).Set(c.From)
			until := new(big.Int).Add(from, PaginationAmount)
			chanRequests <- request{c.ChainId, c.Rpc, from, until}
		}
	}()
	for completed := 0; completed < len(Chains); {
		// Aggregate and send everything else out.
		c := <-chanSuccesses
		fromBlocks[c].Add(fromBlocks[c], PaginationAmount)
		if blockHeights[c].Cmp(fromBlocks[c]) <= 0 {
			completed++
			slog.Info("done", "chain id", c)
			continue
		}
		// Since we haven't hit the limit yet of what we know the height is,
		// continue.
		until := new(big.Int).Add(fromBlocks[c], PaginationAmount)
		if until.Cmp(blockHeights[c]) > 0 {
			until = blockHeights[c]
		}
		chanRequests <- request{
			chainId: c,
			rpc:     rpcs[c],
			from:    fromBlocks[c],
			until:   until,
		}
	}
}

func getDeployments(rpc string, chainId uint64, from, until *big.Int) (deployments []ContractDeployment, err error) {
	var buf bytes.Buffer
	type param struct {
		FromBlock string   `json:"fromBlock"`
		ToBlock   string   `json:"toBlock"`
		Topics    []string `json:"topics"`
		Address   string   `json:"address"`
	}
	err = json.NewEncoder(&buf).Encode(struct {
		Id      string  `json:"id"`
		Method  string  `json:"method"`
		Params  []param `json:"params"`
		Jsonrpc string  `json:"jsonrpc"`
	}{
		Id:     "1",
		Method: "eth_getLogs",
		Params: []param{{
			FromBlock: "0x" + from.Text(16),
			ToBlock:   "0x" + until.Text(16),
			Topics:    []string{"0xc0e812780707128d9a180db8ee4d1c1f1300b6dd0626d577b5d9ac759b76253c"},
			Address:   AddrPrecompile,
		}},
		Jsonrpc: "2.0",
	})
	if err != nil {
		return nil, fmt.Errorf("encode: %v", err)
	}
	resp, err := http.Post(rpc, "application/json", &buf)
	if err != nil {
		return nil, fmt.Errorf("rpc: %v", err)
	}
	defer resp.Body.Close()
	// We can reuse the buffer from earlier, since it should've been drained.
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("drain response: %v", err)
	}
	buf2 := buf
	var reply struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Topics []string `json:"topics"`
		Result []struct {
			Address         string   `json:"address"`
			Topics          []string `json:"topics"`
			Data            string   `json:"data"`
			BlockNumber     string   `json:"blockNumber"`
			TransactionHash string   `json:"transactionHash"`
			BlockHash       string   `json:"blockHash"`
		} `json:"result"`
	}
	if err := json.NewDecoder(&buf).Decode(&reply); err != nil {
		return nil, fmt.Errorf("decode resp: %v", err)
	}
	switch err := reply.Error; {
	case err.Code != "":
		return nil, fmt.Errorf("error from rpc (%#v): %v", buf2.String(), err)
	}
	for _, r := range reply.Result {
		// We're not going to do much checking here to keep things relatively simple.
		codeHash := r.Topics[1]
		data, err := hex.DecodeString(strings.TrimPrefix(r.Data, "0x"))
		if err != nil {
			return nil, fmt.Errorf("decode hex: %v", err)
		}
		a, err := abi.Unpack("ProgramActivated", data)
		if err != nil {
			return nil, fmt.Errorf("decode event data: %v", err)
		}
		moduleHash, ok := a[0].([32]byte)
		if !ok {
			return nil, fmt.Errorf("modulehash: %T", a[0])
		}
		program, ok := a[1].(ethCommon.Address)
		if !ok {
			return nil, fmt.Errorf("program: %T", a[1])
		}
		dataFee, ok := a[2].(*big.Int)
		if !ok {
			return nil, fmt.Errorf("datafee: %T", a[2])
		}
		version, ok := a[3].(uint16)
		if !ok {
			return nil, fmt.Errorf("version: %T", a[3])
		}
		deployments = append(deployments, ContractDeployment{
			ChainId:         chainId,
			BlockNumber:     r.BlockNumber,
			Address:         "0x" + hex.EncodeToString(program[:]),
			CodeHash:        codeHash,
			ModuleHash:      "0x" + hex.EncodeToString(moduleHash[:]),
			TransactionHash: r.TransactionHash,
			BlockHash:       r.BlockHash,
			DataFee:         dataFee.String(),
			Version:         int(version),
		})
	}
	return
}

func getBlockHeights() (blockHeights map[uint64]*big.Int) {
	blockHeights = make(map[uint64]*big.Int, len(Chains))
	blockHeightResps := make(chan blockHeightResp)
	for _, c := range Chains {
		go func(chainId uint64, rpc string) {
			r, err := http.Post(rpc, "application/json", strings.NewReader(`
{"jsonrpc":"2.0","id":1,"method":"eth_getBlockByNumber","params":["latest", true]}`,
			))
			if err != nil {
				log.Fatalf("lookup chain id %v: %v", chainId, err)
			}
			defer r.Body.Close()
			var buf bytes.Buffer
			var resp struct {
				Result struct {
					Transactions []struct {
						BlockNumber string `json:"blockNumber"`
					} `json:"transactions"`
				} `json:"result"`
			}
			if _, err := buf.ReadFrom(r.Body); err != nil {
				log.Fatalf("read buf %v: %v", chainId, err)
			}
			buf2 := buf
			if err := json.NewDecoder(&buf).Decode(&resp); err != nil {
				log.Fatalf("decode: %v: %v", chainId, err)
			}
			if len(resp.Result.Transactions) == 0 {
				log.Fatalf("bad resp: %v: %#v", chainId, buf2.String())
			}
			maxBlock := new(big.Int)
			for _, tx := range resp.Result.Transactions {
				blockHeight, ok := new(big.Int).SetString(
					strings.TrimPrefix(tx.BlockNumber, "0x"),
					16,
				)
				if !ok {
					log.Fatalf("bad block height: %v: %v", chainId, tx.BlockNumber)
				}
				if maxBlock.Cmp(blockHeight) < 0 {
					maxBlock = blockHeight
				}
			}
			blockHeightResps <- blockHeightResp{chainId, maxBlock}
		}(c.ChainId, c.Rpc)
	}
	for range Chains {
		h := <-blockHeightResps
		// Make a copy so we don't keep the other goroutine alive completely.
		blockHeights[h.chainId] = new(big.Int).Set(h.cur)
	}
	return blockHeights
}

func getFromBlocks(db *sql.DB) (fromBlocks map[uint64]*big.Int) {
	fromBlocks = make(map[uint64]*big.Int, len(Chains))
	for _, c := range Chains {
		fromBlocks[c.ChainId] = new(big.Int).Set(c.From)
	}
	rows, err := db.Query("SELECT chain_id, block_number FROM stylus_ingestor_checkpoints_1")
	if err != nil {
		log.Fatalf("query checkpoints: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			chainId     uint64
			blockNumber string
		)
		if err := rows.Scan(&chainId, &blockNumber); err != nil {
			log.Fatalf("scan from blocks: %v", err)
		}
		n, ok := new(big.Int).SetString(blockNumber, 10)
		if !ok {
			log.Fatalf("scan block number: %v", blockNumber)
		}
		fromBlocks[chainId] = n
	}
	return fromBlocks
}

func init() {
	r := csv.NewReader(bytes.NewReader(chainsB))
	r.Comma = '	'
	for i := 1; ; i++ {
		x, err := r.Read()
		switch err {
		case io.EOF:
			return
		case nil:
			// Do nothing
		default:
			panic(err)
		}
		if i == 1 {
			continue // Assume header.
		}
		from, ok := new(big.Int).SetString(x[3], 10)
		if !ok {
			panic(fmt.Sprintf("tsv %v: %v", i, x[3]))
		}
		chainId, err := strconv.ParseUint(x[4], 10, 64)
		if err != nil {
			panic(fmt.Sprintf("chain %v: id: %v", i, err))
		}
		Chains = append(Chains, Chain{
			Name:    x[0],
			Website: x[1],
			Rpc:     x[2],
			From:    from,
			ChainId: chainId,
		})
	}
}
