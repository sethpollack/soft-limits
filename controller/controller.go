package controller

import (
	"context"
	"log"
	"time"

	"github.com/sethpollack/soft-limits/heapster"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/pkg/api"
)

const (
	softLimitCpuAnnotation = "sethpollack.net/soft-limit-cpu"
	softLimitMemAnnotation = "sethpollack.net/soft-limit-memory"
)

type softLimitController struct {
	podClient      corev1.PodsGetter
	heapsterClient *heapster.HeapsterMetricsClient
}

func NewController(client *kubernetes.Clientset) *softLimitController {
	return &softLimitController{
		podClient:      client.CoreV1(),
		heapsterClient: heapster.NewHeapsterMetricsClient(client.CoreV1()),
	}
}

func (c *softLimitController) Run(ctx context.Context, duration time.Duration) {
	for {
		select {
		case <-ctx.Done():
		case <-time.After(duration * time.Second):
			c.killPods()
		}
	}
}

func (c *softLimitController) killPods() {
	pods, err := c.podClient.Pods("").List(metav1.ListOptions{})
	if err != nil {
		log.Println(err)
		return
	}

	for _, pod := range pods.Items {
		podLimits, hasLimits := getPodSoftLimits(pod)
		if !hasLimits {
			continue
		}

		podMetrics, err := c.getPodMetrics(pod)
		if err != nil {
			log.Println(err)
			continue
		}

		if ok := lessThan(podLimits, podMetrics); ok {
			log.Printf("Killing pod %s-%s", pod.Name, pod.Namespace)
			err := c.podClient.Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{})
			if err != nil {
				log.Println(err)
				continue
			}
		}
	}
}

func (c *softLimitController) getPodMetrics(pod v1.Pod) (api.ResourceList, error) {
	podMetrics := api.ResourceList{}
	containerMetrics, err := c.heapsterClient.GetPodMetrics(pod)
	if err != nil {
		return podMetrics, err
	}

	for _, cm := range containerMetrics.Containers {
		cpu := *podMetrics.Cpu()
		cpu.Add(*cm.Usage.Cpu())
		podMetrics[api.ResourceCPU] = cpu

		mem := *podMetrics.Memory()
		mem.Add(*cm.Usage.Memory())
		podMetrics[api.ResourceMemory] = mem
	}

	return podMetrics, nil
}

func getPodSoftLimits(p v1.Pod) (api.ResourceList, bool) {
	cpuLimit, hasCpuLimit := p.Annotations[softLimitCpuAnnotation]
	memLimit, hasMemLimit := p.Annotations[softLimitMemAnnotation]

	podLimits := api.ResourceList{}
	if !hasCpuLimit && !hasMemLimit {
		return podLimits, false
	}

	if hasCpuLimit {
		cpuQuantity, err := resource.ParseQuantity(cpuLimit)
		if err == nil {
			podLimits[api.ResourceCPU] = cpuQuantity
		}
	}

	if hasMemLimit {
		memQuantity, err := resource.ParseQuantity(memLimit)
		if err == nil {
			podLimits[api.ResourceMemory] = memQuantity
		}
	}

	return podLimits, true
}

func lessThan(a api.ResourceList, b api.ResourceList) bool {
	result := true
	for key, value := range a {
		if other, found := b[key]; found {
			if other.Cmp(value) < 0 {
				result = false
			}
		}
	}

	return result
}
