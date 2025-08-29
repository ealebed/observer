package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/jackc/pgx/v5/pgxpool"
	ctrl "sigs.k8s.io/controller-runtime"
)

type ServiceReconciler struct {
	client.Client
	DB          *pgxpool.Pool
	TableName   string
	ClusterName string
}

func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("service", req.NamespacedName)

	// Try to get the Service; if it's gone, wipe rows for {cluster, ns, service}
	var svc corev1.Service
	err := r.Get(ctx, req.NamespacedName, &svc)
	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}
	if err != nil { // NotFound → delete rows
		tbl := sanitizeTableIdent(r.TableName)
		q := fmt.Sprintf(`DELETE FROM %s WHERE cluster=$1 AND namespace=$2 AND service=$3`, tbl)
		if _, derr := r.DB.Exec(ctx, q, r.ClusterName, req.Namespace, req.Name); derr != nil {
			return ctrl.Result{}, derr
		}
		logger.V(1).Info("pruned rows for deleted service")
		return ctrl.Result{}, nil
	}

	// Service still exists → nothing to do; EndpointSlice controller handles adds/updates.
	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}, builder.WithPredicates()).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

var _ = types.NamespacedName{}
