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
	"github.com/jackc/pgx/v5/pgxpool"
	ctrl "sigs.k8s.io/controller-runtime"
)

type EndpointSliceReconciler struct {
	client.Client
	DB            *pgxpool.Pool
	Log           logr.Logger
	LabelSelector string
	RequeueAfter  time.Duration
	TableName     string
	ClusterName   string
}

func (r *EndpointSliceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("slice", req.NamespacedName)

	// Try to get the slice; if it's gone, we can't know the service from the name alone.
	// The Service controller will handle the full prune on service deletion.
	var es discoveryv1.EndpointSlice
	if err := r.Get(ctx, req.NamespacedName, &es); err != nil {
		return ctrl.Result{RequeueAfter: r.RequeueAfter}, client.IgnoreNotFound(err)
	}

	// Optional label filter "k=v[,k=v]" against the EndpointSlice labels
	if r.LabelSelector != "" && !matchKV(es.Labels, r.LabelSelector) {
		return ctrl.Result{RequeueAfter: r.RequeueAfter}, nil
	}

	service := es.Labels[discoveryv1.LabelServiceName]
	if service == "" {
		return ctrl.Result{RequeueAfter: r.RequeueAfter}, nil
	}

	// ---- NEW: union across *all* EndpointSlices for this service in this namespace ----
	var list discoveryv1.EndpointSliceList
	if err := r.List(ctx, &list,
		client.InNamespace(es.Namespace),
		client.MatchingLabels(map[string]string{discoveryv1.LabelServiceName: service}),
	); err != nil {
		return ctrl.Result{}, err
	}

	type row struct{ UID, Name, IP string }
	desired := map[string]row{}

	for _, sl := range list.Items {
		// keep LabelSelector semantics: skip non-matching slices
		if r.LabelSelector != "" && !matchKV(sl.Labels, r.LabelSelector) {
			continue
		}
		for _, ep := range sl.Endpoints {
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
				uid = fmt.Sprintf("%s/%s/%s", sl.Namespace, service, ip)
			}
			desired[uid] = row{UID: uid, Name: name, IP: ip}
		}
	}

	// Upsert & prune in a single transaction
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tbl := sanitizeTableIdent(r.TableName)

	for _, e := range desired {
		q := fmt.Sprintf(`
		  INSERT INTO %s (cluster, namespace, service, pod_uid, pod_name, pod_ip, ready, last_seen)
		  VALUES ($1,$2,$3,$4,$5,$6,true, now())
		  ON CONFLICT (cluster, namespace, service, pod_uid)
		  DO UPDATE SET pod_ip = EXCLUDED.pod_ip, ready = true, last_seen = now()`, tbl)
		if _, err := tx.Exec(ctx, q,
			r.ClusterName, es.Namespace, service, e.UID, e.Name, e.IP); err != nil {
			return ctrl.Result{}, err
		}
	}

	uids := make([]string, 0, len(desired))
	for uid := range desired {
		uids = append(uids, uid)
	}

	// prune any rows for this {cluster,namespace,service} that are no longer present
	qDel := fmt.Sprintf(`
	  DELETE FROM %s
	  WHERE cluster = $1 AND namespace = $2 AND service = $3
	    AND pod_uid <> ALL($4)`, tbl)
	if _, err := tx.Exec(ctx, qDel, r.ClusterName, es.Namespace, service, uids); err != nil {
		return ctrl.Result{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ctrl.Result{}, err
	}

	logger.V(1).Info("synced endpoints",
		"cluster", r.ClusterName, "namespace", es.Namespace, "service", service, "count", len(desired))
	return ctrl.Result{RequeueAfter: r.RequeueAfter}, nil
}

func (r *EndpointSliceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1.EndpointSlice{}, builder.WithPredicates()).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

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

var _ = types.NamespacedName{}
