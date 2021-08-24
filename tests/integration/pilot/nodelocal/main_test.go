//go:build integ
// +build integ

// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nodelocal

import (
	"istio.io/istio/pkg/test/framework/components/echo"
	pilotcommont "istio.io/istio/pkg/test/framework/components/echo/common"
	"istio.io/istio/pkg/test/framework/components/echo/echoboot"
	"istio.io/istio/pkg/test/framework/components/namespace"
	"istio.io/istio/pkg/test/framework/label"
	"istio.io/istio/pkg/test/scopes"
	"istio.io/istio/pkg/test/util/retry"
	"istio.io/istio/tests/integration/pilot/common"
	"testing"
	"time"

	"istio.io/istio/pkg/test/framework"
	"istio.io/istio/pkg/test/framework/components/istio"
	"istio.io/istio/pkg/test/framework/resource"
)

const (
	nodeLocalSvcA = "node-local-svc-A"
	nodeLocalSvcB = "node-local-svc-B"
	nodeLocalSvcC = "node-local-svc-C"
)

var (
	i             istio.Instance
	testNamespace string
	echos         echo.Instances
	apps          = &common.EchoDeployments{}

	retryTimeout = retry.Timeout(1 * time.Minute)
)

// TestMain defines the entrypoint for pilot tests using a standard Istio installation.
// just use for this test.
func TestMain(m *testing.M) {
	framework.
		NewSuite(m).
		Label(label.CustomSetup).
		RequireMinClusters(2).
		RequireMinVersion(21).
		Setup(istio.Setup(&i, enableK8sInternalTrafficPolicy)).
		Setup(deployEchos).
		Run()
}

func enableK8sInternalTrafficPolicy(_ resource.Context, cfg *istio.Config) {
	cfg.ControlPlaneValues = `
values:
  pilot:
    env:
      PILOT_ENABLE_KUBERNETES_INTERNAL_TRAFFIC_POLICY: true
`
}

func deployEchos(t resource.Context) error {
	var err error
	// Create a new namespace in each cluster.
	apps.Namespace, err = namespace.New(t, namespace.Config{
		Prefix: "node-local",
		Inject: true,
	})
	if err != nil {
		return err
	}

	primary := t.Clusters().Primaries()[0]
	remote := t.Clusters().Exclude(primary)[0]
	// primary cluster.
	echos, err = echoboot.NewBuilder(t).
		WithClusters(primary).
		WithConfig(echo.Config{
			Service:   nodeLocalSvcA,
			Namespace: apps.Namespace,
			Ports:     pilotcommont.EchoPorts,
		}).
		WithConfig(echo.Config{
			Service:               nodeLocalSvcC,
			Namespace:             apps.Namespace,
			Ports:                 pilotcommont.EchoPorts,
			InternalTrafficPolicy: "Local",
		}).Build()
	apps.PodA = echos.Match(echo.Service(nodeLocalSvcA))
	apps.PodC = echos.Match(echo.Service(nodeLocalSvcC))

	// remote cluster
	echos, err = echoboot.NewBuilder(t).
		WithClusters(remote).
		WithConfig(echo.Config{
			Service:   nodeLocalSvcB,
			Namespace: apps.Namespace,
			Ports:     pilotcommont.EchoPorts,
		}).Build()
	apps.PodB = echos.Match(echo.Service(nodeLocalSvcB))

	return err
}
