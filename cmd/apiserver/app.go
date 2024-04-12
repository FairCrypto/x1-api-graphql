// Package main implements the API server entry point.
package main

import (
	"context"
	"fantom-api-graphql/cmd/apiserver/build"
	"fantom-api-graphql/internal/config"
	"fantom-api-graphql/internal/graphql/resolvers"
	"fantom-api-graphql/internal/handlers"
	"fantom-api-graphql/internal/logger"
	"fantom-api-graphql/internal/repository"
	"fantom-api-graphql/internal/svc"
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	cache "github.com/victorspringer/http-cache"
	"github.com/victorspringer/http-cache/adapter/memory"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// apiServer implements the API server application
type apiServer struct {
	cfg          *config.Config
	log          logger.Logger
	api          resolvers.ApiResolver
	srv          *http.Server
	closed       chan interface{}
	isVersionReq bool
}

var (
	offlineValidatorsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "graphql_validators_offline",
		Help: "The total number of offline validators",
	})
	notVotingValidatorsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "graphql_validators_not_voting",
		Help: "The total number of not voting validators",
	})
	cheaterValidatorsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "graphql_validators_cheater",
		Help: "The total number of cheater validators",
	})
	totalValidatorsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "graphql_validators_total",
		Help: "The total number of validators",
	})
)

// init initializes the API server
func (app *apiServer) init() {
	// make sure to capture version request and rescan depth
	flag.BoolVar(&app.isVersionReq, "v", false, "get the application version")

	// get the configuration including parsing the calling flags
	var err error
	app.cfg, err = config.Load()
	if nil != err {
		log.Fatal(err)
		return
	}

	// configure logger based on the configuration
	app.log = logger.New(app.cfg)
	app.closed = make(chan interface{})

	// make sure to pass logger and config to internals
	repository.SetConfig(app.cfg)
	repository.SetLogger(app.log)
	resolvers.SetConfig(app.cfg)
	resolvers.SetLogger(app.log)
	svc.SetConfig(app.cfg)
	svc.SetLogger(app.log)

	go app.RunValidatorChecks()

	// make the HTTP server
	app.makeHttpServer()

}

// run executes the API server function.
func (app *apiServer) run() {
	// print the app version and exit if this is the only thing requested
	build.PrintVersion(app.cfg)
	if app.isVersionReq {
		return
	}

	// make sure to capture terminate signals
	app.observeSignals()

	// run services
	svc.Manager().Run()

	// start responding to requests
	app.log.Infof("welcome to Fantom GraphQL API server")
	app.log.Infof("listening for requests on %s", app.cfg.Server.BindAddress)

	// listen the interface
	err := app.srv.ListenAndServe()
	if err != nil {
		app.log.Errorf(err.Error())
	}

	// waiting for terminate to finish
	<-app.closed
}

// makeHttpServer creates and configures the HTTP server to be used to serve incoming requests
func (app *apiServer) makeHttpServer() {
	// create request MUXer
	srvMux := new(http.ServeMux)

	// create HTTP server to handle our requests
	app.srv = &http.Server{
		Addr:              app.cfg.Server.BindAddress,
		ReadTimeout:       time.Second * time.Duration(app.cfg.Server.ReadTimeout),
		WriteTimeout:      time.Second * time.Duration(app.cfg.Server.WriteTimeout),
		IdleTimeout:       time.Second * time.Duration(app.cfg.Server.IdleTimeout),
		ReadHeaderTimeout: time.Second * time.Duration(app.cfg.Server.HeaderTimeout),
		Handler:           srvMux,
	}

	// setup handlers
	app.setupHandlers(srvMux)
}

// setupHandlers initializes an array of handlers for our HTTP API end-points.
func (app *apiServer) setupHandlers(mux *http.ServeMux) {
	// create root resolver
	app.api = resolvers.New()

	// setup GraphQL API handler
	h := http.TimeoutHandler(
		handlers.Api(app.cfg, app.log, app.api),
		time.Second*time.Duration(app.cfg.Server.ResolverTimeout),
		"Service timeout.",
	)

	memcached, err := memory.NewAdapter(
		memory.AdapterWithAlgorithm(memory.LRU),
		memory.AdapterWithCapacity(10000000),
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	cacheClient, err := cache.NewClient(
		cache.ClientWithAdapter(memcached),
		cache.ClientWithTTL(1*time.Minute),
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	mux.Handle("/api", h)
	mux.Handle("/graphql", h)

	mux.Handle("/health", handlers.Health(app.log, app.cfg))

	// setup gas price estimator REST API resolver
	mux.Handle("/json/gas", handlers.GasPrice(app.log))
	mux.Handle("/json/validators/down", cacheClient.Middleware(handlers.ValidatorsDownJSONHandler(app.log)))
	mux.Handle("/html/validators/down", cacheClient.Middleware(handlers.ValidatorsDownHandler(app.log)))
	mux.Handle("/validators", cacheClient.Middleware(handlers.ValidatorsJSONHandler(app.api, app.log)))
	mux.Handle("/validators/", cacheClient.Middleware(handlers.ValidatorJSONHandler(app.log)))

	// handle GraphiQL interface
	mux.Handle("/graphi", handlers.GraphiHandler(app.cfg.Server.DomainAddress, app.log))
}

// observeSignals setups terminate signals observation.
func (app *apiServer) observeSignals() {
	// log what we do
	app.log.Info("os signals captured")

	// make the signal consumer
	ts := make(chan os.Signal, 1)
	signal.Notify(ts, syscall.SIGINT, syscall.SIGTERM)

	// start monitoring
	go func() {
		defer func() {
			app.terminate()
			close(app.closed)
		}()

		// wait for the signal
		<-ts

		// terminate HTTP responder
		app.log.Notice("closing HTTP server")

		ct, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := app.srv.Shutdown(ct); err != nil {
			app.log.Errorf("could not terminate HTTP listener; %s", err.Error())
		}

		// we closed
		cancel()
		app.log.Notice("HTTP server closed")
	}()
}

// terminate modules of the API server.
func (app *apiServer) terminate() {
	// close resolvers
	app.log.Notice("closing resolver")
	app.api.Close()

	// terminate observers, scanners and dispatchers, etc.
	app.log.Notice("closing services")
	if mgr := svc.Manager(); mgr != nil {
		mgr.Close()
	}

	// terminate connections to DB, blockchain, etc.
	app.log.Notice("closing repository")
	if repo := repository.R(); repo != nil {
		repo.Close()
	}

	app.log.Notice("terminated")
}

func (app *apiServer) RunValidatorChecks() {
	validatorStatusGauges := make(map[uint64]prometheus.Gauge)

	// run the validator checks
	for {
		if app.api == nil {
			time.Sleep(5 * time.Second)
			continue
		}

		// get the list of validators
		validatorStatuses, err := app.api.ValidatorStatuses()
		if err != nil {
			app.log.Errorf("can not get validators list; %s", err.Error())
			time.Sleep(5 * time.Second)
			continue
		}

		// check each validator
		offlineVals := 0
		notVoting := 0
		cheaterVals := 0
		for _, validatorStatus := range validatorStatuses {
			if validatorStatus.IsCheater {
				cheaterVals++
			} else if validatorStatus.IsOffline || validatorStatus.IsWithdrawn {
				offlineVals++
			} else if !validatorStatus.IsVoting {
				notVoting++
			}

			if _, ok := validatorStatusGauges[validatorStatus.Id]; !ok {
				validatorStatusGauges[validatorStatus.Id] = promauto.NewGauge(prometheus.GaugeOpts{
					Name: fmt.Sprintf("graphql_validator_status"),
					Help: fmt.Sprintf("The status of validator"),
					ConstLabels: map[string]string{
						"id": fmt.Sprintf("%d", validatorStatus.Id),
					},
				})
			}

			validatorStatusGauges[validatorStatus.Id].Set(float64(validatorStatus.Status))
		}

		offlineValidatorsGauge.Set(float64(offlineVals))
		notVotingValidatorsGauge.Set(float64(notVoting))
		cheaterValidatorsGauge.Set(float64(cheaterVals))
		totalValidatorsGauge.Set(float64(len(validatorStatuses)))

		// wait for the next check
		time.Sleep(30 * time.Second)
	}
}
