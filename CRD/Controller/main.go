package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Printf("Error building kubeconfig: %v\n", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error creating clientset: %v\n", err)
		os.Exit(1)
	}

	crdWatcher, err := clientset.YourdomainV1().PodPerNodes("default").Watch(context.TODO(), v1.ListOptions{})
	if err != nil {
		fmt.Printf("Error watching custom resource: %v\n", err)
		os.Exit(1)
	}

	for event := range crdWatcher.ResultChan() {
		if event.Type == watch.Added || event.Type == watch.Modified {
			podPerNode, ok := event.Object.(*v1alpha1.PodPerNode)
			if !ok {
				fmt.Printf("Unexpected object type: %T\n", event.Object)
				continue
			}

			// Retrieve the deployment specified in the custom resource
			deployment, err := clientset.AppsV1().Deployments(podPerNode.Namespace).Get(context.TODO(), podPerNode.Spec.DeploymentName, v1.GetOptions{})
			if err != nil {
				fmt.Printf("Error retrieving deployment: %v\n", err)
				continue
			}

			// Get the number of pods for the deployment in the node
			pods, err := clientset.CoreV1().Pods(podPerNode.Namespace).List(context.TODO(), v1.ListOptions{
				LabelSelector: fmt.Sprintf("app=%s", deployment.Name),
				FieldSelector: fmt.Sprintf("spec.nodeName=%s", podPerNode.Spec.NodeName),
			})
			if err != nil {
				fmt.Printf("Error retrieving pods: %v\n", err)
				continue
			}

			// If there are three pods for the deployment in the node, delete the custom resource
			if len(pods.Items) >= 3 {
				err = clientset.YourdomainV1().PodPerNodes(podPerNode.Namespace).Delete(context.TODO(), podPerNode.Name, v1.DeleteOptions{})
				if err != nil {
					fmt.Printf("Error deleting custom resource: %v\n", err)
					continue
				}
				fmt.Printf("Deleted custom resource %s\n", podPerNode.Name)
			}
		}
	}
}
