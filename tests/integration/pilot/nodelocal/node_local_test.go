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
	"istio.io/istio/pkg/test/framework/components/echo/common"
	"istio.io/istio/pkg/test/framework/components/echo/echoboot"
	pilotcommon "istio.io/istio/tests/integration/pilot/common"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"istio.io/istio/pkg/test/echo/common/scheme"
	"istio.io/istio/pkg/test/framework"
	kubecluster "istio.io/istio/pkg/test/framework/components/cluster/kube"
	"istio.io/istio/pkg/test/framework/components/echo"
)

func TestNodeLocal(t *testing.T) {
	framework.
		NewTest(t).
		Features("traffic.locality").
		RequiresSingleCluster().
		Run(func(t framework.TestContext) {
			var err error
			cluster := t.Clusters().Default().(*kubecluster.Cluster)
			if !cluster.MinKubeVersion(21) {
				t.Skipf("k8s version not supported for %s (<%s)", t.Name(), "1.21")
			}
			node, err := cluster.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				t.Fatalf("get node failed == %v", err)
			}
			if len(node.Items) < 2 {
				t.Skipf("Skipping %q: number of node in cluster is %d less 2, skip this test.", t.Name(), len(node.Items))
			}

			builder := echoboot.NewBuilder(t, cluster).
				WithConfig(echo.Config{
					Service:           pilotcommon.PodASvc,
					Namespace:         apps.Namespace,
					Ports:             common.EchoPorts,
					Subsets:           []echo.SubsetConfig{{}},
					Locality:          "region.zone.subzone",
					WorkloadOnlyPorts: common.WorkloadPorts,
					PodAffinity: map[string]string{
						"app": pilotcommon.PodCSvc,
					},
				}).
				WithConfig(echo.Config{
					Service:           pilotcommon.PodBSvc,
					Namespace:         apps.Namespace,
					Ports:             common.EchoPorts,
					Subsets:           []echo.SubsetConfig{{}},
					WorkloadOnlyPorts: common.WorkloadPorts,
					PodAntiAffinity: map[string]string{
						"app": pilotcommon.PodCSvc,
					},
				}).
				WithConfig(echo.Config{
					Service:               pilotcommon.PodCSvc,
					Namespace:             apps.Namespace,
					Ports:                 common.EchoPorts,
					Subsets:               []echo.SubsetConfig{{}},
					WorkloadOnlyPorts:     common.WorkloadPorts,
					InternalTrafficPolicy: "Local",
				})

			echos, err := builder.Build()
			if err != nil {
				t.Fatalf("create deployments failed")
			}
			apps.All = echos
			apps.PodA = echos.Match(echo.Service(pilotcommon.PodASvc))
			apps.PodB = echos.Match(echo.Service(pilotcommon.PodBSvc))
			apps.PodC = echos.Match(echo.Service(pilotcommon.PodCSvc))
			time.Sleep(5 * time.Second)
			// on same node can access service C when service C set NodeLocalTraffic=Local
			apps.PodA[0].CallWithRetryOrFail(t, echo.CallOptions{
				Port:      &echo.Port{ServicePort: 80},
				Scheme:    scheme.HTTP,
				Address:   fmt.Sprintf("c.%s.svc.cluster.local", apps.Namespace.Name()),
				Validator: echo.ExpectOK(),
			})
			apps.PodB[0].CallWithRetryOrFail(t, echo.CallOptions{
				Port:      &echo.Port{ServicePort: 80},
				Scheme:    scheme.HTTP,
				Address:   fmt.Sprintf("c.%s.svc.cluster.local", apps.Namespace.Name()),
				Validator: echo.ExpectCode("503"),
			})
		})
}
