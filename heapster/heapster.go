package heapster

import (
	"encoding/json"
	"fmt"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	metricsapi "k8s.io/metrics/pkg/apis/metrics/v1alpha1"
)

var (
	prefix       = "/apis"
	groupVersion = fmt.Sprintf("%s/%s", metricsGv.Group, metricsGv.Version)
	metricsRoot  = fmt.Sprintf("%s/%s", prefix, groupVersion)
	metricsGv    = schema.GroupVersion{Group: "metrics", Version: "v1alpha1"}
)

type HeapsterMetricsClient struct {
	SVCClient         corev1.ServicesGetter
	HeapsterNamespace string
	HeapsterScheme    string
	HeapsterService   string
	HeapsterPort      string
}

func NewHeapsterMetricsClient(svcClient corev1.ServicesGetter) *HeapsterMetricsClient {
	return &HeapsterMetricsClient{
		SVCClient:         svcClient,
		HeapsterNamespace: "kube-system",
		HeapsterScheme:    "http",
		HeapsterService:   "heapster",
		HeapsterPort:      "",
	}
}

func podMetricsUrl(p v1.Pod) string {
	return fmt.Sprintf("%s/namespaces/%s/pods/%s", metricsRoot, p.Namespace, p.Name)
}

func (cli *HeapsterMetricsClient) GetPodMetrics(pod v1.Pod) (podMetrics metricsapi.PodMetrics, err error) {
	path := podMetricsUrl(pod)

	params := map[string]string{"labelSelector": ""}

	resultRaw, err := getHeapsterMetrics(cli, path, params)
	if err != nil {
		return podMetrics, err
	}

	err = json.Unmarshal(resultRaw, &podMetrics)
	if err != nil {
		return podMetrics, fmt.Errorf("failed to unmarshall heapster response: %v", err)
	}

	return podMetrics, nil
}

func getHeapsterMetrics(cli *HeapsterMetricsClient, path string, params map[string]string) ([]byte, error) {
	return cli.
		SVCClient.
		Services(cli.HeapsterNamespace).
		ProxyGet(cli.HeapsterScheme, cli.HeapsterService, cli.HeapsterPort, path, params).
		DoRaw()
}
