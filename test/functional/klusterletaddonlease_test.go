// Copyright (c) 2020 Red Hat, Inc.

// +build functional

package functional

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Lease", func() {
	BeforeEach(func() {
		SetDefaultEventuallyTimeout(20 * time.Second)
		SetDefaultEventuallyPollingInterval(1 * time.Second)
	})

	It("Create Lease", func() {
		// Skip("Skip have to fix")
		By("Creating the secret", func() {
			b, err := ioutil.ReadFile(filepath.Clean(hubKubeConfig))
			//Create secret
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hub-config-secret",
					Namespace: addonNamespace,
				},
				Data: map[string][]byte{
					"kubeconfig": b,
				},
				Type: corev1.SecretTypeOpaque,
			}

			_, err = clientManagedCluster.CoreV1().Secrets(addonNamespace).Create(context.TODO(), secret, metav1.CreateOptions{})
			Expect(err).To(BeNil())
		})
		var l0 *coordinationv1.Lease
		When("Secret created check lease", func() {

			Eventually(func() error {
				var err error
				klog.Infof("Wait for lease %s/%s", leaseName, clusterNamespace)
				l0, err = clientHub.CoordinationV1().Leases(clusterNamespace).Get(context.TODO(), leaseName, metav1.GetOptions{})
				return err
			}).Should(BeNil())
		})
		When("Lease created check renew", func() {
			Eventually(func() bool {
				klog.Infof("Wait for lease to be renewed %s/%s", leaseName, clusterNamespace)
				l1, err := clientHub.CoordinationV1().Leases(clusterNamespace).Get(context.TODO(), leaseName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return l0.Spec.RenewTime != l1.Spec.RenewTime
			}).Should(BeTrue())
		})
	})
})
