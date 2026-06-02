// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"errors"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
)

// The providerID contains the serverID and looks like:
// openstack:///352378e0-7610-45c4-bfb4-9ad973ef8652
const providerPrefix = "openstack:///"

func getServerIDFromNode(node *corev1.Node) (string, error) {
	providerStr := node.Spec.ProviderID
	//nolint:staticcheck
	if serverID := strings.TrimLeft(providerStr, providerPrefix); serverID != "" {
		return serverID, nil
	}
	return "", errors.New("serverID not found in provider ID")
}

func getLabelValue(obj any, lblKey string) (string, bool) {
	objMeta, err := meta.Accessor(obj)
	if err != nil {
		return "", false
	}

	lbl := objMeta.GetLabels()
	if lbl == nil {
		return "", false
	}

	val, ok := lbl[lblKey]
	return val, ok
}
