//###############################################################################
//# Copyright (c) 2020 Red Hat, Inc.
//###############################################################################

package controllers

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/openshift/library-go/pkg/operator/events"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	fakekubeclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLeaseReconciler_waitPodRunning(t *testing.T) {
	s := scheme.Scheme
	podRunning := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podRunning",
			Namespace: "pod-namespace",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	podFailed := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podFailed",
			Namespace: "pod-namespace",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
		},
	}
	c := fake.NewFakeClientWithScheme(s, podRunning, podFailed)
	type fields struct {
		Client               client.Client
		Log                  logr.Logger
		Scheme               *runtime.Scheme
		LeaseName            string
		LeaseNamespace       string
		HubConfigSecretName  string
		LeaseDurationSeconds int32
		PodName              string
		PodNamespace         string
		leaseUpdater         *leaseUpdater
	}
	tests := []struct {
		name    string
		fields  fields
		want    bool
		wantErr bool
	}{
		{
			name: "Pod Running",
			fields: fields{
				Client:       c,
				PodName:      "podRunning",
				PodNamespace: "pod-namespace",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Pod Failed",
			fields: fields{
				Client:       c,
				PodName:      "podFailed",
				PodNamespace: "pod-namespace",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "Pod Not found",
			fields: fields{
				Client:       c,
				PodName:      "podNotFound",
				PodNamespace: "pod-namespace",
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &LeaseReconciler{
				Client:               tt.fields.Client,
				Log:                  tt.fields.Log,
				Scheme:               tt.fields.Scheme,
				LeaseName:            tt.fields.LeaseName,
				LeaseNamespace:       tt.fields.LeaseNamespace,
				HubConfigSecretName:  tt.fields.HubConfigSecretName,
				LeaseDurationSeconds: tt.fields.LeaseDurationSeconds,
				PodName:              tt.fields.PodName,
				PodNamespace:         tt.fields.PodNamespace,
				leaseUpdater:         tt.fields.leaseUpdater,
			}
			got, err := r.waitPodRunning()
			if (err != nil) != tt.wantErr {
				t.Errorf("LeaseReconciler.waitPodReady() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("LeaseReconciler.waitPodReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLeaseReconciler_newUpdaterLease(t *testing.T) {
	emptySecret := &corev1.Secret{}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret1",
			Namespace: "secret-namespace",
		},
		Data: map[string][]byte{
			"kubeconfig": []byte(
				`
apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: https://fake.com:6443
  name: default-cluster
contexts:
- context:
    cluster: default-cluster
    namespace: default
    user: default-auth
  name: default-context
current-context: default-context
kind: Config
preferences: {}
users:
- name: default-auth
  user:
    token: fake
`),
		},
		Type: corev1.SecretTypeOpaque,
	}
	c := fake.NewFakeClient([]runtime.Object{}...)
	hubClient, err := BuildKubeClientWithSecret(secret)
	if err != nil {
		t.Error(err)
	}
	type fields struct {
		Client                    client.Client
		Log                       logr.Logger
		Scheme                    *runtime.Scheme
		LeaseName                 string
		LeaseNamespace            string
		HubConfigSecretName       string
		BuildKubeClientWithSecret IBuildKubeClientWithSecret
		LeaseDurationSeconds      int32
		PodName                   string
		PodNamespace              string
		leaseUpdater              *leaseUpdater
	}
	type args struct {
		instance *corev1.Secret
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *leaseUpdater
		wantErr bool
	}{
		{
			name: "succeed",
			fields: fields{
				Client:                    c,
				BuildKubeClientWithSecret: BuildKubeClientWithSecret,
				LeaseName:                 "lease-name",
				LeaseNamespace:            "lease-namespace",
			},
			args: args{
				instance: secret,
			},
			want: &leaseUpdater{
				hubClient: hubClient,
				name:      "lease-name",
				namespace: "lease-namespace",
			},
			wantErr: false,
		},
		{
			name: "failed",
			fields: fields{
				Client:                    c,
				BuildKubeClientWithSecret: BuildKubeClientWithSecret,
				LeaseName:                 "lease-name",
				LeaseNamespace:            "lease-namespace",
			},
			args: args{
				instance: emptySecret,
			},
			want: &leaseUpdater{
				hubClient: hubClient,
				name:      "lease-name",
				namespace: "lease-namespace",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &LeaseReconciler{
				Client:                        tt.fields.Client,
				Log:                           tt.fields.Log,
				Scheme:                        tt.fields.Scheme,
				LeaseName:                     tt.fields.LeaseName,
				LeaseNamespace:                tt.fields.LeaseNamespace,
				HubConfigSecretName:           tt.fields.HubConfigSecretName,
				BuildKubeClientWithSecretFunc: tt.fields.BuildKubeClientWithSecret,
				LeaseDurationSeconds:          tt.fields.LeaseDurationSeconds,
				PodName:                       tt.fields.PodName,
				PodNamespace:                  tt.fields.PodNamespace,
				leaseUpdater:                  tt.fields.leaseUpdater,
			}
			got, err := r.newUpdaterLease(tt.args.instance)
			if (err != nil) != tt.wantErr {
				t.Errorf("LeaseReconciler.newUpdaterLease() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if !reflect.DeepEqual(got.name, tt.want.name) {
					t.Errorf("LeaseReconciler.newUpdaterLease() = %v, want %v", got.name, tt.want.name)
				}
				if !reflect.DeepEqual(got.namespace, tt.want.namespace) {
					t.Errorf("LeaseReconciler.newUpdaterLease() = %v, want %v", got.namespace, tt.want.namespace)
				}
			}
			// if !reflect.DeepEqual(got, tt.want) {
			// 	t.Errorf("LeaseReconciler.newUpdaterLease() = %v, want %v", got, tt.want)
			// }
		})
	}
}

func Test_leaseUpdater_start(t *testing.T) {
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lease-name",
			Namespace: "lease-namespace",
		},
	}
	var leaseDurationSeconds int32 = 1
	cNotFound := fakekubeclient.NewSimpleClientset()
	cFound := fakekubeclient.NewSimpleClientset(lease)
	type fields struct {
		hubClient kubernetes.Interface
		namespace string
		name      string
	}
	type args struct {
		ctx                  context.Context
		leaseDurationSeconds *int32
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Lease not exists",
			fields: fields{
				hubClient: cNotFound,
				name:      "lease-name",
				namespace: "lease-namespace",
			},
			args: args{
				ctx:                  context.TODO(),
				leaseDurationSeconds: &leaseDurationSeconds,
			},
			wantErr: false,
		},
		{
			name: "Lease exists",
			fields: fields{
				hubClient: cFound,
				name:      "lease-name",
				namespace: "lease-namespace",
			},
			args: args{
				ctx:                  context.TODO(),
				leaseDurationSeconds: &leaseDurationSeconds,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &leaseUpdater{
				hubClient: tt.fields.hubClient,
				namespace: tt.fields.namespace,
				name:      tt.fields.name,
			}
			if err := u.start(tt.args.ctx, tt.args.leaseDurationSeconds); (err != nil) != tt.wantErr {
				t.Errorf("leaseUpdater.start() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				time.Sleep(time.Duration(*tt.args.leaseDurationSeconds) * time.Second)
				time.Sleep(1 * time.Second)
				l0, err := u.hubClient.CoordinationV1().Leases(u.namespace).Get(context.TODO(), u.name, metav1.GetOptions{})
				if err != nil {
					t.Errorf("Lease not found %s/%s", u.name, u.namespace)
				}
				time.Sleep(time.Duration(*tt.args.leaseDurationSeconds) * time.Second)
				time.Sleep(1 * time.Second)
				l1, err := u.hubClient.CoordinationV1().Leases(u.namespace).Get(context.TODO(), u.name, metav1.GetOptions{})
				if err != nil {
					t.Errorf("Lease not found %s/%s", u.name, u.namespace)
				}
				if l0.Spec.RenewTime == l1.Spec.RenewTime {
					t.Error("Lease not updated")
				}
			}
		})
	}
}

func Test_leaseUpdater_stop(t *testing.T) {
	updateCtx, cancel := context.WithCancel(context.TODO())
	type fields struct {
		hubClient kubernetes.Interface
		namespace string
		name      string
		cancel    context.CancelFunc
		recorder  events.Recorder
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "succeed",
			fields: fields{
				name:      "lease-name",
				namespace: "lease-namespace",
				cancel:    cancel,
			},
			args: args{
				ctx: updateCtx,
			},
		},
		{
			name: "succeed cancel nil",
			fields: fields{
				name:      "lease-name",
				namespace: "lease-namespace",
				cancel:    nil,
			},
			args: args{
				ctx: updateCtx,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &leaseUpdater{
				hubClient: tt.fields.hubClient,
				namespace: tt.fields.namespace,
				name:      tt.fields.name,
				cancel:    tt.fields.cancel,
				recorder:  tt.fields.recorder,
			}
			u.stop(tt.args.ctx)
			if u.cancel != nil {
				t.Error("u.cancel must be nil")
			}
		})
	}
}

const (
	leaseName      = "lease"
	leaseNamespace = "lease-ns"
	podName        = "pod"
	podNamespace   = "pod-ns"
)

func TestLeaseReconciler_Reconcile(t *testing.T) {
	s := scheme.Scheme
	s.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Namespace{})
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: leaseNamespace,
		},
	}
	secret := &corev1.Secret{}
	now := metav1.NewTime(time.Now())

	secretDelete := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			DeletionTimestamp: &now,
		},
	}
	podRunning := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: podNamespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	podFailed := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: podNamespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
		},
	}
	cSecret := fake.NewFakeClientWithScheme(s, ns, secret)
	cWithPodRunning := fake.NewFakeClientWithScheme(s, ns, secret, podRunning)
	cWithPodFailed := fake.NewFakeClientWithScheme(s, ns, secret, podFailed)
	cWithoutSecret := fake.NewFakeClientWithScheme(s, ns)
	cSecretDeleted := fake.NewFakeClientWithScheme(s, ns, secretDelete)
	type fields struct {
		Client                    client.Client
		Log                       logr.Logger
		Scheme                    *runtime.Scheme
		LeaseName                 string
		LeaseNamespace            string
		HubConfigSecretName       string
		BuildKubeClientWithSecret IBuildKubeClientWithSecret
		LeaseDurationSeconds      int32
		PodName                   string
		PodNamespace              string
		leaseUpdater              *leaseUpdater
	}
	type args struct {
		req ctrl.Request
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    ctrl.Result
		wantErr bool
	}{
		{
			name: "succeed",
			fields: fields{
				Client:                    cSecret,
				Log:                       ctrl.Log.WithName("controllers").WithName("Lease"),
				Scheme:                    s,
				LeaseName:                 leaseName,
				LeaseNamespace:            leaseNamespace,
				HubConfigSecretName:       "fakesecretname",
				LeaseDurationSeconds:      1,
				BuildKubeClientWithSecret: fakeBuikdBuildKubeClientWithSecret,
			},
			want:    ctrl.Result{},
			wantErr: false,
		},
		{
			name: "succeed no secret",
			fields: fields{
				Client:                    cWithoutSecret,
				Log:                       ctrl.Log.WithName("controllers").WithName("Lease"),
				Scheme:                    s,
				LeaseName:                 leaseName,
				LeaseNamespace:            leaseNamespace,
				HubConfigSecretName:       "fakesecretname",
				LeaseDurationSeconds:      1,
				BuildKubeClientWithSecret: fakeBuikdBuildKubeClientWithSecret,
			},
			want:    ctrl.Result{},
			wantErr: false,
		},
		{
			name: "succeed delete",
			fields: fields{
				Client:                    cSecretDeleted,
				Log:                       ctrl.Log.WithName("controllers").WithName("Lease"),
				Scheme:                    s,
				LeaseName:                 leaseName,
				LeaseNamespace:            leaseNamespace,
				HubConfigSecretName:       "fakesecretname",
				LeaseDurationSeconds:      1,
				BuildKubeClientWithSecret: fakeBuikdBuildKubeClientWithSecret,
			},
			want:    ctrl.Result{},
			wantErr: false,
		},
		{
			name: "succeed with padname and padnamespace",
			fields: fields{
				Client:                    cWithPodRunning,
				Log:                       ctrl.Log.WithName("controllers").WithName("Lease"),
				Scheme:                    s,
				LeaseName:                 leaseName,
				LeaseNamespace:            leaseNamespace,
				HubConfigSecretName:       "fakesecretname",
				PodName:                   podName,
				PodNamespace:              podNamespace,
				LeaseDurationSeconds:      1,
				BuildKubeClientWithSecret: fakeBuikdBuildKubeClientWithSecret,
			},
			want:    ctrl.Result{},
			wantErr: false,
		},
		{
			name: "succeed with padname and padnamespace pod failed",
			fields: fields{
				Client:                    cWithPodFailed,
				Log:                       ctrl.Log.WithName("controllers").WithName("Lease"),
				Scheme:                    s,
				LeaseName:                 leaseName,
				LeaseNamespace:            leaseNamespace,
				HubConfigSecretName:       "fakesecretname",
				PodName:                   podName,
				PodNamespace:              podNamespace,
				LeaseDurationSeconds:      1,
				BuildKubeClientWithSecret: fakeBuikdBuildKubeClientWithSecret,
			},
			want:    ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &LeaseReconciler{
				Client:                        tt.fields.Client,
				Log:                           tt.fields.Log,
				Scheme:                        tt.fields.Scheme,
				LeaseName:                     tt.fields.LeaseName,
				LeaseNamespace:                tt.fields.LeaseNamespace,
				HubConfigSecretName:           tt.fields.HubConfigSecretName,
				BuildKubeClientWithSecretFunc: tt.fields.BuildKubeClientWithSecret,
				LeaseDurationSeconds:          tt.fields.LeaseDurationSeconds,
				PodName:                       tt.fields.PodName,
				PodNamespace:                  tt.fields.PodNamespace,
				leaseUpdater:                  tt.fields.leaseUpdater,
			}
			got, err := r.Reconcile(tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("LeaseReconciler.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LeaseReconciler.Reconcile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func fakeBuikdBuildKubeClientWithSecret(secret *corev1.Secret) (kubernetes.Interface, error) {
	return fakekubeclient.NewSimpleClientset(), nil
}
