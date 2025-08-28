package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	ctrl "sigs.k8s.io/controller-runtime"
)

type EndpointSliceReconciler struct {
	client.Client
	DB            *pgxpool.Pool
	Log           logr.Logger
	LabelSelector string
	RequeueAfter  time.Duration
	TableName     string // <- configurable target table (optionally schema-qualified)
}

func (r *EndpointSliceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("slice", req.NamespacedName)

	var es discoveryv1.EndpointSlice
	if err := r.Get(ctx, req.NamespacedName, &es); err != nil {
		// slice deleted: periodic resync will prune DB
		return ctrl.Result{RequeueAfter: r.RequeueAfter}, client.IgnoreNotFound(err)
	}

	// optional label filter "k=v[,k=v]"
	if r.LabelSelector != "" && !matchKV(es.Labels, r.LabelSelector) {
		return ctrl.Result{RequeueAfter: r.RequeueAfter}, nil
	}

	service := es.Labels[discoveryv1.LabelServiceName]
	if service == "" {
		return ctrl.Result{RequeueAfter: r.RequeueAfter}, nil
	}

	type row struct{ UID, Name, IP string }
	desired := map[string]row{}

	for _, ep := range es.Endpoints {
		if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
			continue
		}
		if len(ep.Addresses) == 0 {
			continue
		}

		ip := ep.Addresses[0]
		uid := ""
		name := ""
		if ep.TargetRef != nil && ep.TargetRef.Kind == "Pod" {
			uid = string(ep.TargetRef.UID)
			name = ep.TargetRef.Name
		}
		if uid == "" {
			uid = fmt.Sprintf("%s/%s/%s", es.Namespace, service, ip)
		}
		desired[uid] = row{UID: uid, Name: name, IP: ip}
	}

	// Upsert & prune in a single transaction
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Best-effort rollback; harmless after a successful Commit.
	defer func() { _ = tx.Rollback(ctx) }()

	// Safely-quoted identifier for the (optional) schema-qualified table name.
	tblIdent := sanitizeTableIdent(r.TableName)

	for _, e := range desired {
		q := fmt.Sprintf(`
		  INSERT INTO %s(service, namespace, pod_uid, pod_name, ip, last_seen)
		  VALUES ($1,$2,$3,$4,$5, now())
		  ON CONFLICT (service, pod_uid)
		  DO UPDATE SET ip = EXCLUDED.ip, last_seen = now()`, tblIdent)
		if _, err := tx.Exec(ctx, q,
			service, es.Namespace, e.UID, e.Name, e.IP); err != nil {
			return ctrl.Result{}, err
		}
	}

	uidList := make([]string, 0, len(desired))
	for uid := range desired {
		uidList = append(uidList, uid)
	}

	// prune rows for this {service,namespace} no longer present
	qDel := fmt.Sprintf(`
	  DELETE FROM %s
	  WHERE service = $1 AND namespace = $2
	    AND pod_uid <> ALL($3)`, tblIdent)
	if _, err := tx.Exec(ctx, qDel,
		service, es.Namespace, uidList); err != nil {
		return ctrl.Result{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ctrl.Result{}, err
	}

	logger.V(1).Info("synced endpoints", "service", service, "namespace", es.Namespace, "count", len(desired))
	return ctrl.Result{RequeueAfter: r.RequeueAfter}, nil
}

func (r *EndpointSliceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1.EndpointSlice{}, builder.WithPredicates()).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// trivial "k=v[,k=v]" matcher (kept simple)
func matchKV(lbls map[string]string, sel string) bool {
	for _, p := range strings.Split(sel, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			return false
		}
		if lbls[kv[0]] != kv[1] {
			return false
		}
	}
	return true
}

// force import
var _ = types.NamespacedName{}

// sanitizeTableIdent returns a safely-quoted identifier suitable for SQL (supports "schema.table").
// If name is empty, it defaults to "server".
func sanitizeTableIdent(name string) string {
	if name == "" {
		name = "server"
	}
	parts := strings.Split(name, ".")
	return pgx.Identifier(parts).Sanitize() // -> "schema"."table" or "server"
}
