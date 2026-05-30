// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"net"
	"time"
)

// Options for the controller.
type Options struct {
	*Auth
	ConfigPath             string
	KubeConfig             string
	Threadiness            int
	IsDebug                bool
	RecheckInterval        time.Duration
	MetricHost             net.IP
	MetricPort             int
	DefaultFloatingNetwork string
	DefaultFloatingSubnet  string
}
