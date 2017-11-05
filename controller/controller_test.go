package controller

import (
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/api"
)

func TestLimitsExceeded(t *testing.T) {
	testCases := map[string]struct {
		limits         api.ResourceList
		actual         api.ResourceList
		limitsExceeded bool
	}{
		"equalMemory": {
			limits: api.ResourceList{
				api.ResourceMemory: resource.MustParse("100Mi"),
			},
			actual: api.ResourceList{
				api.ResourceMemory: resource.MustParse("100Mi"),
			},
			limitsExceeded: false,
		},
		"equalCpu": {
			limits: api.ResourceList{
				api.ResourceCPU: resource.MustParse("100m"),
			},
			actual: api.ResourceList{
				api.ResourceCPU: resource.MustParse("100m"),
			},
			limitsExceeded: false,
		},
		"lessMemory": {
			limits: api.ResourceList{
				api.ResourceMemory: resource.MustParse("0Mi"),
			},
			actual: api.ResourceList{
				api.ResourceMemory: resource.MustParse("100Mi"),
			},
			limitsExceeded: true,
		},
		"lessCpu": {
			limits: api.ResourceList{
				api.ResourceCPU: resource.MustParse("0m"),
			},
			actual: api.ResourceList{
				api.ResourceCPU: resource.MustParse("100m"),
			},
			limitsExceeded: true,
		},
		"greaterMemory": {
			limits: api.ResourceList{
				api.ResourceMemory: resource.MustParse("100Mi"),
			},
			actual: api.ResourceList{
				api.ResourceMemory: resource.MustParse("0Mi"),
			},
			limitsExceeded: false,
		},
		"greaterCpu": {
			limits: api.ResourceList{
				api.ResourceCPU: resource.MustParse("100m"),
			},
			actual: api.ResourceList{
				api.ResourceCPU: resource.MustParse("0m"),
			},
			limitsExceeded: false,
		},
	}

	for testName, testCase := range testCases {
		if result := limitsExceeded(testCase.limits, testCase.actual); result != testCase.limitsExceeded {
			t.Errorf("%s expected: %v, actual: %v, limits=%v, actual=%v", testName, testCase.limitsExceeded, result, testCase.limits, testCase.actual)
		}
	}
}

func equals(a api.ResourceList, b api.ResourceList) bool {
	return a.Memory().Value() == b.Memory().Value() &&
		a.Cpu().MilliValue() == b.Cpu().MilliValue()
}

func TestGetPodSoftLimits(t *testing.T) {
	testCases := map[string]struct {
		pod            v1.Pod
		expectedLimits api.ResourceList
		hasLimits      bool
	}{
		"noAnnotations": {
			pod:            v1.Pod{},
			expectedLimits: api.ResourceList{},
			hasLimits:      false,
		},
		"annotationsMemory": {
			pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						softLimitMemAnnotation: "100Mi",
					},
				},
			},
			expectedLimits: api.ResourceList{
				api.ResourceMemory: resource.MustParse("100Mi"),
			},
			hasLimits: true,
		},
		"annotationsCpu": {
			pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						softLimitCpuAnnotation: "100m",
					},
				},
			},
			expectedLimits: api.ResourceList{
				api.ResourceCPU: resource.MustParse("100m"),
			},
			hasLimits: true,
		},
		"annotationsMemoryPercent": {
			pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						softLimitMemAnnotation: "10%",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Resources: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								v1.ResourceMemory: resource.MustParse("100Mi"),
							},
						}},
					},
				},
			},
			expectedLimits: api.ResourceList{
				api.ResourceMemory: resource.MustParse("90Mi"),
			},
			hasLimits: true,
		},
		"annotationsCpuPercent": {
			pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						softLimitCpuAnnotation: "10%",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Resources: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								v1.ResourceCPU: resource.MustParse("100m"),
							},
						}},
					},
				},
			},
			expectedLimits: api.ResourceList{
				api.ResourceCPU: resource.MustParse("90m"),
			},
			hasLimits: true,
		},
		"invalidAnnotationsMemory": {
			pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						softLimitMemAnnotation: "10%%",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Resources: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								v1.ResourceMemory: resource.MustParse("100Mi"),
							},
						}},
					},
				},
			},
			expectedLimits: api.ResourceList{
				api.ResourceMemory: resource.MustParse("100Mi"),
			},
			hasLimits: true,
		},
		"invalidAnnotationsCpu": {
			pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						softLimitCpuAnnotation: "10%%",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Resources: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								v1.ResourceCPU: resource.MustParse("100m"),
							},
						}},
					},
				},
			},
			expectedLimits: api.ResourceList{
				api.ResourceCPU: resource.MustParse("100m"),
			},
			hasLimits: true,
		},
		"missingLimitsMemory": {
			pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						softLimitMemAnnotation: "10%",
					},
				},
			},
			expectedLimits: api.ResourceList{},
			hasLimits:      false,
		},
		"missingLimitsCpu": {
			pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						softLimitCpuAnnotation: "10%",
					},
				},
			},
			expectedLimits: api.ResourceList{},
			hasLimits:      false,
		},
	}

	for testName, testCase := range testCases {
		if actualLimits, _ := getPodSoftLimits(&testCase.pod); !equals(testCase.expectedLimits, actualLimits) {
			t.Errorf("%s expected: %v, actual: %v", testName, testCase.expectedLimits, actualLimits)
		}
	}
}
