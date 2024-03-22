// Package handlers hold an HTTP/WS handlers chain along with separate middleware implementations.
package handlers

import (
	"embed"
	"encoding/json"
	"fantom-api-graphql/internal/config"
	"fantom-api-graphql/internal/logger"
	"fantom-api-graphql/internal/repository"
	"fantom-api-graphql/internal/types"
	"io"
	"math/big"
	"net/http"
	"text/template"
)

//go:embed templates
var htmlTemplates embed.FS

// GasPrice constructs and return the REST API HTTP handler for Gas Price provider.
func GasPrice(log logger.Logger) http.Handler {
	// build the handler function
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get the gas price estimation
		val, err := repository.R().GasPriceExtended()
		if err != nil {
			log.Critical("can not get gas price; %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// respond
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(val)
		if err != nil {
			log.Critical("can not encode gas price structure; %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
}

// ValidatorsDownHandler provides a handler for a textual list of validators with downtime.
func ValidatorsDownHandler(log logger.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func(Body io.ReadCloser) {
			_ = Body.Close()
		}(r.Body)

		list, err := repository.R().DownValidators()
		if err != nil {
			log.Criticalf("can not get list of offline server; %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		tmp := template.Must(template.ParseFS(htmlTemplates, "templates/down.html"))

		w.Header().Set("Content-Type", "text/html")
		err = tmp.Execute(w, struct {
			Val   []types.OfflineValidator
			Count int
		}{Val: list, Count: len(list)})
		if err != nil {
			log.Criticalf("can not execute HTML templates; %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
}

func Health(log logger.Logger, cfg *config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// check node health check
		res, err := http.Get(cfg.Opera.ApiHealthCheckUrl)
		if err != nil {
			log.Error("x1 node health check failed", "err", err.Error())
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if res.StatusCode != http.StatusOK {
			log.Error("x1 node health check failed", "status", res.StatusCode)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		apiBh, err := repository.R().BlockHeight()
		if err != nil {
			log.Error("Failed to get API block height", "err", err.Error())
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		nodeBh, err := repository.R().LastKnownBlock()
		if err != nil {
			log.Error("Failed to get node block height", "err", err.Error())
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		nodeBhi := new(big.Int).SetUint64(nodeBh)
		apiBhi := apiBh.ToInt()
		diff := big.NewInt(0).Sub(apiBhi, nodeBhi)

		// compare blocks
		if diff.Cmp(big.NewInt(cfg.Opera.BlockDiff)) == 1 {
			log.Error("Block height difference is too big", "diff", diff.String())
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		log.Info("API server is healthy", "nodeBh", nodeBh, "apiBh", apiBh, "diff", diff.String())
		w.WriteHeader(http.StatusOK)
	})
}
