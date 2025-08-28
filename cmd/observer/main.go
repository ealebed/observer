package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ealebed/observer/internal/controller"
	"github.com/ealebed/observer/internal/version"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(discoveryv1.AddToScheme(scheme))
}

func main() {
	// ---- flags & env ----
	var (
		requeueAfter  time.Duration
		labelSelector string
		watchNS       string
		tableName     string
		clusterName   string
	)
	flag.DurationVar(&requeueAfter, "requeue-after", 60*time.Second, "Periodic reconcile interval.")
	flag.StringVar(&labelSelector, "selector", getenv("ENDPOINT_SELECTOR", ""), "EndpointSlice label selector (e.g. 'app=my-svc').")
	flag.StringVar(&watchNS, "namespace", getenv("NAMESPACE", ""), "Namespace to watch (empty = all).")
	flag.StringVar(&tableName, "table", getenv("TABLE_NAME", "server"), "Destination Postgres table (optionally schema-qualified, e.g. 'public.server').")
	flag.StringVar(&clusterName, "cluster", getenv("CLUSTER_NAME", "default"), "Cluster name label to write with each row.")

	zopts := zap.Options{Development: false}
	zopts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zopts)))
	log := ctrl.Log.WithName("observer")
	log.Info("starting",
		"version", version.Version,
		"selector", labelSelector,
		"cluster", clusterName,
		"namespace", watchNS,
		"table", tableName,
	)

	// ---- Postgres ----
	pool, err := newPoolFromEnv(context.Background())
	if err != nil {
		log.Error(err, "postgres connect failed")
		os.Exit(1)
	}
	defer pool.Close()

	// ---- manager options (no HA, no metrics/probes) ----
	opts := ctrl.Options{
		Scheme:                 scheme,
		LeaderElection:         false,
		Metrics:                server.Options{BindAddress: "0"}, // disable metrics server
		HealthProbeBindAddress: "0",                              // disable health/ready probes
	}

	// Optional: scope cache to a single namespace
	if watchNS != "" {
		opts.Cache = cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				watchNS: {},
			},
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), opts)
	if err != nil {
		log.Error(err, "manager start failed")
		os.Exit(1)
	}

	// ---- controller ----
	if err := (&controller.EndpointSliceReconciler{
		Client:        mgr.GetClient(),
		DB:            pool,
		Log:           ctrl.Log.WithName("endpointslice"),
		LabelSelector: labelSelector,
		RequeueAfter:  requeueAfter,
		TableName:     tableName,
		ClusterName:   clusterName,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "controller setup failed")
		os.Exit(1)
	}

	// ---- run ----
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "manager stopped with error")
		os.Exit(1)
	}
}

func newPoolFromEnv(ctx context.Context) (*pgxpool.Pool, error) {
	host := os.Getenv("PGHOST")
	user := os.Getenv("PGUSER")
	pass := os.Getenv("PGPASSWORD")
	db := os.Getenv("PGDATABASE")
	port := getenv("PGPORT", "5432")
	ssl := getenv("PGSSLMODE", "require")

	if host == "" || user == "" || pass == "" || db == "" {
		return nil, fmt.Errorf("missing PG env vars (need PGHOST, PGUSER, PGPASSWORD, PGDATABASE)")
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s pool_max_conns=4",
		host, port, user, pass, db, ssl,
	)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
