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

package pilot

import (
	"context"
	kubecluster "istio.io/istio/pkg/test/framework/components/cluster/kube"
	"istio.io/istio/pkg/test/framework/image"
	kubetest "istio.io/istio/pkg/test/kube"
	"istio.io/istio/pkg/test/util/retry"
	"istio.io/istio/pkg/test/util/tmpl"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"istio.io/istio/pkg/test/framework"
)

const nodeLocalTemplate = `
kind: Service
metadata:
  name: nodeLocal
  labels:
    nodeLocal: a
spec:
  ports:
  - port: 80
    name: http
  selector:
    nodeLocal: a
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nodeLocal
spec:
  replicas: 4
  selector:
    matchLabels:
      nodeLocal: a
  template:
    metadata:
      labels:
         nodeLocal: a
    spec:
      containers:
      - name: istio-proxy
        image: auto
        imagePullPolicy: IfNotPresent
---
`

type NodeLocalInput struct {
	imagePullPolicy string
}

func TestNodeLocal(t *testing.T) {
	framework.
		NewTest(t).
		Features("traffic.locality").
		RequiresSingleCluster().
		Run(func(t framework.TestContext) {
			templateParams := map[string]string{
				"imagePullSecret": image.PullSecretNameOrFail(t),
				"pullPolicy":      image.PullImagePolicy(),
			}

			t.Config().ApplyYAMLOrFail(t, apps.Namespace.Name(), tmpl.MustEvaluate(`apiVersion: v1
kind: Service
metadata:
  name: nodeLocal
  labels:
    nodeLocal: a
spec:
  ports:
  - port: 80
    name: http
  selector:
    nodeLocal: a
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nodeLocal
spec:
  replicas: 4
  selector:
    matchLabels:
      nodeLocal: a
  template:
    metadata:
      labels:
         nodeLocal: a
    spec:
      {{- if ne .imagePullSecret "" }}
      imagePullSecrets:
      - name: {{ .imagePullSecret }}
      {{- end }}
      containers:
      - name: istio-proxy
        image: auto
        imagePullPolicy: {{ .pullPolicy }}
---
`, templateParams))
			cs := t.Clusters().Default().(*kubecluster.Cluster)
			retry.UntilSuccessOrFail(t, func() error {
				_, err := kubetest.CheckPodsAreReady(kubetest.NewPodFetch(cs, apps.Namespace.Name(), "nodeLocal=a"))
				return err
			}, retry.Timeout(time.Minute*2), retry.Delay(time.Second))
			t.Logf("pod is ready ...")
			pods, err := cs.CoreV1().Pods(apps.Namespace.Name()).List(context.TODO(), metav1.ListOptions{LabelSelector: "nodeLocal=a"})
			if err != nil {
				t.Fatalf("get pod failded == %v", err)
			}
			for _, p := range pods.Items {
				t.Logf("get pod success --- %s/%s/%s", p.Name, p.Status.HostIP, p.Status.PodIP)
			}
		})
}
