// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

// (c) Copyright IBM Corporation 2019, 2020. All Rights Reserved.
// Note to U.S. Government Users Restricted Rights:
// U.S. Government Users Restricted Rights - Use, duplication or disclosure restricted by GSA ADP Schedule
// Contract with IBM Corp.
// Licensed Materials - Property of IBM
//

//go:build functional
// +build functional

package functional

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	libgoclient "github.com/stolostron/library-go/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	leaseName             = "addon-lease"
	hubKubeConfig         = "kubeconfig/hub_kind_kubeconfig.yaml"
	hubInternalKubeConfig = "kubeconfig/hub_internal_kind_kubeconfig.yaml"
	clusterNamespace      = "open-cluster-management-self-import"
	addonNamespace        = "open-cluster-management-agent-addon"
)

var (
	clientManagedCluster        kubernetes.Interface
	clientManagedClusterDynamic dynamic.Interface
	clientHub                   kubernetes.Interface
	clientHubDynamic            dynamic.Interface
	gvrSecret                   schema.GroupVersionResource
)

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)
}

var _ = BeforeSuite(func() {
	gvrSecret = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}

	setupManagedCluster()
	setupHub()

})

func TestRcmController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "kusterlet-addon-lease-controller Suite")
}

func setupManagedCluster() {
	var err error
	clientManagedCluster, err = libgoclient.NewDefaultKubeClient("")
	Expect(err).To(BeNil())
	clientManagedClusterDynamic, err = libgoclient.NewDefaultKubeClientDynamic("")
	Expect(err).To(BeNil())
}

func setupHub() {
	var err error
	clientHub, err = libgoclient.NewDefaultKubeClient(hubKubeConfig)
	Expect(err).To(BeNil())
	clientHubDynamic, err = libgoclient.NewDefaultKubeClientDynamic(hubKubeConfig)
	Expect(err).To(BeNil())

	//Create ns
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterNamespace,
		},
	}
	_, err = clientHub.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	Expect(err).To(BeNil())
}
