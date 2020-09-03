// Copyright (c) 2020 Red Hat, Inc.

// +build functional

package functional

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Lease", func() {
	It("Skip", func() {
		Expect(true).Should(BeTrue())
	})
})
