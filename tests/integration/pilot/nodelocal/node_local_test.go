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
	"fmt"
	"testing"

	"istio.io/istio/pkg/test/echo/common/scheme"
	"istio.io/istio/pkg/test/framework"
	"istio.io/istio/pkg/test/framework/components/echo"
)

func TestNodeLocal(t *testing.T) {
	framework.
		NewTest(t).
		Features("traffic.k8s-internal-traffic").
		RequiresMinClusters(2).
		Run(func(t framework.TestContext) {
			// When nodeLocalSvcC sets `InternalTrafficPolicy=Local`, it only can be accessed by the same node instance(nodeLocalSvcA).
			apps.PodA[0].CallWithRetryOrFail(t, echo.CallOptions{
				Port:      &echo.Port{ServicePort: 80},
				Scheme:    scheme.HTTP,
				Address:   fmt.Sprintf("%s.%s.svc.cluster.local", nodeLocalSvcC, apps.Namespace.Name()),
				Validator: echo.ExpectOK(),
			})

			apps.PodB[0].CallWithRetryOrFail(t, echo.CallOptions{
				Port:      &echo.Port{ServicePort: 80},
				Scheme:    scheme.HTTP,
				Address:   fmt.Sprintf("%s.%s.svc.cluster.local", nodeLocalSvcC, apps.Namespace.Name()),
				Validator: echo.ExpectCode("503"),
			})
		})
}
