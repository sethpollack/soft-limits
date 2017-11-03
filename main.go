package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"time"

	"github.com/sethpollack/soft-limits/controller"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig *string
	interval   *int
)

func init() {
	if home := os.Getenv("HOME"); home != "/" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	interval = flag.Int("interval", 10, "frequency for checking pods")
}

func main() {
	flag.Parse()

	var (
		config *rest.Config
		err    error
	)

	if *kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		os.Exit(1)
	}

	client := kubernetes.NewForConfigOrDie(config)
	ctrl := controller.NewController(client)
	ctrl.Run(context.Background(), time.Duration(*interval))
}
