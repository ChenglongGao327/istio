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
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"istio.io/istio/pkg/test/echo/common/scheme"
	"istio.io/istio/pkg/test/framework"
	"istio.io/istio/pkg/test/framework/components/echo"
	pilotcommont "istio.io/istio/pkg/test/framework/components/echo/common"
	"istio.io/istio/pkg/test/framework/components/echo/echoboot"
	"istio.io/istio/pkg/test/framework/components/istio"
	"istio.io/istio/pkg/test/framework/components/namespace"
	"istio.io/istio/pkg/test/framework/label"
	"istio.io/istio/pkg/test/framework/resource"
	"istio.io/istio/tests/integration/pilot/common"
)

const (
	nodeLocalSvcA = "node-local-svc-a"
	nodeLocalSvcB = "node-local-svc-b"
	nodeLocalSvcC = "node-local-svc-c"
)

var (
	i    istio.Instance
	apps = &common.EchoDeployments{}
)

// TestMain defines the entrypoint for pilot tests using a standard Istio installation.
// just use for this test.
func TestMain(m *testing.M) {
	framework.
		NewSuite(m).
		Label(label.CustomSetup).
		RequireMinClusters(1).
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
	var echos echo.Instances
	// Create a new namespace in each cluster.
	apps.Namespace, err = namespace.New(t, namespace.Config{
		Prefix: "node-local",
		Inject: true,
	})
	if err != nil {
		return err
	}

	primary := t.Clusters().Primaries()[0]
	//remote := t.Clusters().Exclude(primary)[0]
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
	if err != nil {
		return err
	}
	apps.PodA = echos.Match(echo.Service(nodeLocalSvcA))
	apps.PodC = echos.Match(echo.Service(nodeLocalSvcC))

	// remote cluster
	echos, err = echoboot.NewBuilder(t).
		WithClusters(primary).
		WithConfig(echo.Config{
			Service:   nodeLocalSvcB,
			Namespace: apps.Namespace,
			Ports:     pilotcommont.EchoPorts,
		}).Build()
	apps.PodB = echos.Match(echo.Service(nodeLocalSvcB))

	return err
}

func TestNodeLocal(t *testing.T) {
	framework.
		NewTest(t).
		Features("traffic.k8s-internal-traffic").
		RequiresMinClusters(1).
		Run(func(t framework.TestContext) {
			primary := t.Clusters().Primaries()[0]
			svc, _ := primary.CoreV1().Services(apps.Namespace.Name()).List(context.TODO(), metav1.ListOptions{LabelSelector: "app=node-local-svc-c"})
			if svc != nil {
				for i, v := range svc.Items {
					t.Logf("svc== %d===%+v", i, v)
				}
			}
			deploy, _ := primary.AppsV1().Deployments("istio-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "app=istiod"})
			if deploy != nil {
				for i, v := range deploy.Items {
					t.Logf("deploy=== %d===%+v", i, v.Spec.Template.Spec.Containers[0].Env)
				}
			}

			t.Logf("start node local test")
			// When nodeLocalSvcC sets `InternalTrafficPolicy=Local`, it only can be accessed by the same node instance(nodeLocalSvcA).
			res, _ := apps.PodA[0].CallWithRetry(echo.CallOptions{
				Port:      &echo.Port{ServicePort: 80},
				Scheme:    scheme.HTTP,
				Address:   fmt.Sprintf("%s.%s.svc.cluster.local", nodeLocalSvcC, apps.Namespace.Name()),
				Validator: echo.ExpectOK(),
			})
			t.Logf("===res==%+v", res)
			t.Logf("it can be accessed by the same node")
			apps.PodB[0].CallWithRetryOrFail(t, echo.CallOptions{
				Port:      &echo.Port{ServicePort: 80},
				Scheme:    scheme.HTTP,
				Address:   fmt.Sprintf("%s.%s.svc.cluster.local", nodeLocalSvcC, apps.Namespace.Name()),
				Validator: echo.ExpectCode("503"),
			})
			t.Logf("it can not be accessed by the different node")
		})
}
