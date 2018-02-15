package util

import (
	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetOutOfClusterClient(configPath, masterurl string) kubernetes.Interface {
	glog.Infoln("Initializing out-of-cluster kube client")
	config, err := clientcmd.BuildConfigFromFlags(masterurl, configPath)
	if err != nil {
		glog.Fatalln(err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalln(err)
	}
	return client
}

func GetInClusterClient() kubernetes.Interface {
	glog.Infoln("Initializing in-cluster kube client")
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatalln(err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalln(err)
	}
	return client
}
