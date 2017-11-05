package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/sethpollack/soft-limits/controller"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig *string
	interval   *int
	namespace  *string
)

func init() {
	if home := os.Getenv("HOME"); home != "/" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	if ns := os.Getenv("MY_POD_NAMESPACE"); ns != "" {
		namespace = flag.String("namespace", ns, "(optional) controller namespace.")
	} else {
		namespace = flag.String("namespace", "", "controller namespace.")
	}

	interval = flag.Int("interval", 10, "(optional) run frequency")
}

func main() {
	flag.Parse()

	var (
		config *rest.Config
		err    error
	)

	if *namespace == "" {
		log.Printf("Missing namespace")
		os.Exit(1)
	}

	if *kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		os.Exit(1)
	}

	ctx := context.Background()
	client := kubernetes.NewForConfigOrDie(config)
	sharedInformers := informers.NewSharedInformerFactory(client, 10*time.Minute)
	ctrl := controller.NewController(client, sharedInformers.Core().V1().Pods(), *namespace)
	sharedInformers.Start(ctx.Done())
	ctrl.Run(ctx, time.Duration(*interval))
}
