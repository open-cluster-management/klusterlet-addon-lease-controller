//###############################################################################
//# Copyright (c) 2020 Red Hat, Inc.
//###############################################################################

package controllers

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	leaseLog = ctrl.Log.WithName("lease-controller")
)

// IBuildKubeClientWithSecret a function which convert a secret to client
type IBuildKubeClientWithSecret func(secret *corev1.Secret) (kubernetes.Interface, error)

// LeaseReconciler reconciles a Secret object
type LeaseReconciler struct {
	client.Client
	Log                 logr.Logger
	Scheme              *runtime.Scheme
	LeaseName           string
	LeaseNamespace      string
	HubConfigSecretName string
	// Use a type because this allows to create a fake function
	BuildKubeClientWithSecretFunc IBuildKubeClientWithSecret
	LeaseDurationSeconds          int32
	PodName                       string
	PodNamespace                  string
	leaseUpdater                  *leaseUpdater
}

// leaseUpdater periodically updates the lease of a managed cluster
type leaseUpdater struct {
	hubClient         kubernetes.Interface
	namespace         string
	name              string
	lock              sync.Mutex
	cancel            context.CancelFunc
	checkPodIsRunning func() (bool, error) // callback function for checking if pod is running
}

func (r *LeaseReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("lease", req.NamespacedName)

	leaseLog.Info(fmt.Sprintf("processing %s", req.NamespacedName.Name))

	if r.leaseUpdater == nil {
		ready, err := r.checkPodIsRunning()
		if err != nil {
			return reconcile.Result{}, err
		}
		if !ready {
			leaseLog.Info(fmt.Sprintf("Wait until pod %s/%s is ready", r.PodName, r.PodNamespace))
			return reconcile.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
		}
	}

	instance := &corev1.Secret{}

	if err := r.Get(
		context.TODO(),
		types.NamespacedName{Namespace: req.Namespace, Name: req.Name},
		instance,
	); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			if r.leaseUpdater == nil {
				return reconcile.Result{}, nil
			}
			r.leaseUpdater.stop(context.TODO())
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if r.leaseUpdater == nil {
		u, err := r.newUpdaterLease(instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		r.leaseUpdater = u
		err = r.leaseUpdater.start(context.TODO(), &r.LeaseDurationSeconds)
		if err != nil {
			r.leaseUpdater = nil
			return reconcile.Result{}, err
		}
	}

	if instance.DeletionTimestamp != nil {
		leaseLog.Info(fmt.Sprintf("stop lease for %s", req.NamespacedName.Name))
		r.leaseUpdater.stop(context.TODO())
		return reconcile.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *LeaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(r.newSecretPredicate()).
		Complete(r)
}

func (r *LeaseReconciler) newSecretPredicate() predicate.Predicate {
	return predicate.Predicate(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Meta.GetName() == r.HubConfigSecretName
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Meta.GetName() == r.HubConfigSecretName
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.MetaNew.GetName() == r.HubConfigSecretName
		},
	})
}

//checkPodIsRunning check if the pod is ready
func (r *LeaseReconciler) checkPodIsRunning() (bool, error) {
	if r.PodName == "" || r.PodNamespace == "" {
		return true, nil
	}
	pod := corev1.Pod{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: r.PodName, Namespace: r.PodNamespace}, &pod)
	if err != nil {
		return false, err
	}
	return pod.Status.Phase == corev1.PodRunning, nil
}

func (r *LeaseReconciler) newUpdaterLease(instance *corev1.Secret) (*leaseUpdater, error) {
	clientset, err := r.BuildKubeClientWithSecretFunc(instance)
	if err != nil {
		leaseLog.Error(err, "kubernetes.NewForConfig")
		return nil, err
	}
	leaseLog.V(2).Info("kubernetes.NewForConfig succeeded")
	return &leaseUpdater{
		hubClient:         clientset,
		name:              r.LeaseName,
		namespace:         r.LeaseNamespace,
		checkPodIsRunning: r.checkPodIsRunning,
	}, nil
}

func BuildKubeClientWithSecret(secret *corev1.Secret) (kubernetes.Interface, error) {
	tempdir, err := ioutil.TempDir("", "kube")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempdir)

	for key, data := range secret.Data {
		if err := ioutil.WriteFile(path.Join(tempdir, key), data, 0600); err != nil {
			return nil, err
		}
	}
	restConfig, err := clientcmd.BuildConfigFromFlags("", path.Join(tempdir, "kubeconfig"))
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(restConfig)
}

// start a lease update routine to update the lease of a managed cluster periodically.
func (u *leaseUpdater) start(ctx context.Context, leaseDurationSeconds *int32) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	_, err := u.hubClient.CoordinationV1().Leases(u.namespace).Get(context.TODO(), u.name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			leaseLog.Info(fmt.Sprintf("start lease for %s/%s", u.name, u.namespace))
			lease := &coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      u.name,
					Namespace: u.namespace,
				},
				Spec: coordinationv1.LeaseSpec{
					LeaseDurationSeconds: leaseDurationSeconds,
				},
			}
			if _, err := u.hubClient.CoordinationV1().Leases(u.namespace).Create(ctx, lease, metav1.CreateOptions{}); err != nil {
				leaseLog.Error(err, fmt.Sprintf("unable to create addon lease %q/%q on hub cluster", u.name, u.namespace))
				return err
			}
		} else {
			return err
		}
	}

	var updateCtx context.Context

	updateCtx, u.cancel = context.WithCancel(ctx)
	d := time.Duration(*leaseDurationSeconds) * time.Second
	go wait.JitterUntilWithContext(updateCtx, u.update, d, -1, true)
	leaseLog.V(2).Info(fmt.Sprintf("ManagedClusterLeaseUpdateStrated Start to update lease %q/%q on hub cluster", u.name, u.namespace))
	return nil
}

// update the lease of a given managed cluster.
func (u *leaseUpdater) update(ctx context.Context) {
	if u.checkPodIsRunning != nil {
		podIsRunning, err := u.checkPodIsRunning()
		if err != nil {
			leaseLog.Error(err, "unable to get pod status")
			return
		}
		if !podIsRunning {
			leaseLog.Info("Skipping lease %s/%s update as pod is not running.")
			return
		}
	}

	leaseLog.Info(fmt.Sprintf("Update lease %s/%s", u.name, u.namespace))
	lease, err := u.hubClient.CoordinationV1().Leases(u.namespace).Get(ctx, u.name, metav1.GetOptions{})
	if err != nil {
		// u.recorder.Eventf("unable to get cluster lease %q/%q on hub cluster %w", u.name, u.namespace, err)
		leaseLog.Error(err, fmt.Sprintf("unable to get cluster lease %q/%q on hub cluster", u.name, u.namespace))
		return
	}

	lease.Spec.RenewTime = &metav1.MicroTime{Time: time.Now()}
	if _, err = u.hubClient.CoordinationV1().Leases(u.namespace).Update(ctx, lease, metav1.UpdateOptions{}); err != nil {
		// u.recorder.Eventf("unable to update addon lease %q/%q on hub cluster %w", u.name, u.namespace, err)
		leaseLog.Error(err, fmt.Sprintf("unable to update cluster lease %q/%q on hub cluster", u.name, u.namespace))
		return
	}
}

// stop the lease update routine.
func (u *leaseUpdater) stop(ctx context.Context) {
	u.lock.Lock()
	defer u.lock.Unlock()
	leaseLog.Info(fmt.Sprintf("stop: Stop to update lease %q/%q on hub cluster", u.name, u.namespace))

	if u.cancel == nil {
		return
	}
	u.cancel()
	u.cancel = nil
}
