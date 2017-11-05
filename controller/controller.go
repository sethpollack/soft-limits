package controller

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/sethpollack/soft-limits/heapster"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	informercorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listercorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api"
)

const (
	softLimitCpuAnnotation = "sethpollack.net/soft-limit-cpu"
	softLimitMemAnnotation = "sethpollack.net/soft-limit-memory"
)

var parsedValues = make(map[string]float64)

type softLimitController struct {
	podClient       corev1.PodsGetter
	podLister       listercorev1.PodLister
	podListerSynced cache.InformerSynced
	heapsterClient  *heapster.HeapsterMetricsClient
	recorder        record.EventRecorder
}

func NewController(client *kubernetes.Clientset, podInformer informercorev1.PodInformer, namespace string) *softLimitController {
	return &softLimitController{
		podClient:       client.CoreV1(),
		podLister:       podInformer.Lister(),
		podListerSynced: podInformer.Informer().HasSynced,
		heapsterClient:  heapster.NewHeapsterMetricsClient(client.CoreV1()),
		recorder:        createEventRecorder(client, namespace),
	}
}

func createEventRecorder(client *kubernetes.Clientset, namespace string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Printf)
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{
		Interface: client.CoreV1().Events(namespace),
	})
	return eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "soft-limit-controller"})
}

func (c *softLimitController) Run(ctx context.Context, duration time.Duration) {
	if !cache.WaitForCacheSync(ctx.Done(), c.podListerSynced) {
		log.Print("timed out waiting for cache sync")
		return
	}

	for {
		select {
		case <-ctx.Done():
		case <-time.After(duration * time.Second):
			c.killPods()
		}
	}
}

func (c *softLimitController) killPods() {
	pods, err := c.podLister.Pods("").List(labels.Everything())
	if err != nil {
		log.Println(err)
		return
	}

	for _, pod := range pods {
		podLimits, hasLimits := getPodSoftLimits(pod)
		if !hasLimits {
			continue
		}

		podMetrics, err := c.getPodMetrics(pod)
		if err != nil {
			log.Println(err)
			continue
		}

		if ok := limitsExceeded(podLimits, podMetrics); ok {
			log.Printf("Killing pod %s-%s", pod.Name, pod.Namespace)
			c.recorder.Event(pod, api.EventTypeNormal, "ExceededSoftLimit", "Killing pod.")
			err := c.podClient.Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{})
			if err != nil {
				log.Println(err)
				continue
			}
		}
	}
}

func (c *softLimitController) getPodMetrics(pod *v1.Pod) (api.ResourceList, error) {
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

func parsePercent(s string) float64 {
	_, ok := parsedValues[s]
	if !ok {
		i, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil {
			log.Println("error parsing percentage", err)
		}
		parsedValues[s] = i
	}

	return parsedValues[s]
}

func calculatePercentage(val int64, percent string) int64 {
	return int64(float64(val) - (float64(val) * (parsePercent(percent) / float64(100))))
}

func getPodSoftLimits(p *v1.Pod) (api.ResourceList, bool) {
	cpuLimit, hasCpuLimit := p.Annotations[softLimitCpuAnnotation]
	memLimit, hasMemLimit := p.Annotations[softLimitMemAnnotation]

	hardLimits := api.ResourceList{}
	if !hasCpuLimit && !hasMemLimit {
		return hardLimits, false
	}

	for _, c := range p.Spec.Containers {
		if hasCpuLimit {
			cpu := *hardLimits.Cpu()
			cpu.Add(*c.Resources.Limits.Cpu())
			hardLimits[api.ResourceCPU] = cpu
		}

		if hasMemLimit {
			mem := *hardLimits.Memory()
			mem.Add(*c.Resources.Limits.Memory())
			hardLimits[api.ResourceMemory] = mem
		}
	}

	softLimits := api.ResourceList{}
	if hasCpuLimit {
		if strings.Contains(cpuLimit, "%") {
			cpu := *hardLimits.Cpu()
			cpu.Set(calculatePercentage(cpu.Value(), cpuLimit))
			if cpu.Value() > 0 {
				softLimits[api.ResourceCPU] = cpu
			}
		} else {
			cpuQuantity, err := resource.ParseQuantity(cpuLimit)
			if err == nil {
				softLimits[api.ResourceCPU] = cpuQuantity
			}
		}
	}

	if hasMemLimit {
		if strings.Contains(memLimit, "%") {
			mem := *hardLimits.Memory()
			mem.Set(calculatePercentage(mem.Value(), memLimit))
			if mem.Value() > 0 {
				softLimits[api.ResourceMemory] = mem
			}
		} else {
			memQuantity, err := resource.ParseQuantity(memLimit)
			if err == nil {
				softLimits[api.ResourceMemory] = memQuantity
			}
		}

	}

	return softLimits, true
}

func limitsExceeded(limits api.ResourceList, actual api.ResourceList) bool {
	exceeded := false
	for resource, limitQuantity := range limits {
		if actualQuantity, found := actual[resource]; found {
			// returns 1 if the actualQuantity is greater than limitQuantity
			if actualQuantity.Cmp(limitQuantity) == 1 {
				exceeded = true
			}
		}
	}
	return exceeded
}
