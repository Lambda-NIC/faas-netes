// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Lambda-NIC/faas-netes/handlers"
	"github.com/Lambda-NIC/faas-netes/types"
	"github.com/Lambda-NIC/faas-netes/version"
	"github.com/Lambda-NIC/faas-provider"
	bootTypes "github.com/Lambda-NIC/faas-provider/types"
	"go.etcd.io/etcd/client"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// LambdaNIC: Create etcd connection for saving distributed values.
const etcdMasterIP string = "127.0.0.1"
const etcdPort string = "2379"

// LambdaNIC: List of SmartNICs to use and how many deployments are there.
var smartNICs = []string{"20.20.20.101", "20.20.20.102",
	"20.20.20.103", "20.20.20.104"}

func initializeEtcd(keysAPI client.KeysAPI) {
	opts := client.SetOptions{Dir: true}
	resp, err := keysAPI.Set(context.Background(),
		"/smartnics",
		"", &opts)
	if err != nil {
		log.Println("SmartNIC directory already exists, cleaning...")
		delopts := client.DeleteOptions{Recursive: true}
		_, err = keysAPI.Delete(context.Background(), "/smartnics", &delopts)
		if err != nil {
			log.Fatal("Error in cleaning smartnic.")
		}
		log.Printf("Deleted smartnic, recreating")
		_, err = keysAPI.Set(context.Background(),
			"/smartnics",
			"", &opts)
		if err != nil {
			log.Fatal("Could not recreate smartNIC")
		}
	} else {
		// print common key info
		log.Printf("Added SmartNIC directory to ETCD. Metadata is %q\n",
			resp)
	}
	resp, err = keysAPI.Set(context.Background(),
		"/deployments",
		"", &opts)
	if err != nil {
		log.Println("Deployments directory already exists, cleaning...")
		delopts := client.DeleteOptions{Recursive: true}
		_, err = keysAPI.Delete(context.Background(), "/deployments", &delopts)
		if err != nil {
			log.Fatal("Error in cleaning deployments.")
		}
		log.Printf("Deleted deployments, recreating")
		_, err = keysAPI.Set(context.Background(),
			"/deployments",
			"", &opts)
		if err != nil {
			log.Fatal("Could not recreate deployments")
		}
	} else {
		// print common key info
		log.Printf("Added Deployments directory to ETCD. Metadata is %q\n",
			resp)
	}
	resp, err = keysAPI.Set(context.Background(),
		"/functions",
		"", &opts)
	if err != nil {
		log.Println("Functions directory already exists, cleaning...")
		delopts := client.DeleteOptions{Recursive: true}
		_, err = keysAPI.Delete(context.Background(), "/functions", &delopts)
		if err != nil {
			log.Fatal("Error in cleaning functions.")
		}
		log.Printf("Deleted functions, recreating")
		_, err = keysAPI.Set(context.Background(),
			"/functions",
			"", &opts)
		if err != nil {
			log.Fatal("Could not recreate functions")
		}
	} else {
		// print common key info
		log.Printf("Added Functions directory to ETCD. Metadata is %q\n",
			resp)
	}

	for _, smartNIC := range smartNICs {
		// Create each smartnic entry.
		resp, err = keysAPI.Set(context.Background(),
			fmt.Sprintf("/smartnics/%s", smartNIC),
			smartNIC, nil)
		if err != nil {
			log.Fatal(err)
		} else {
			// print common key info
			log.Printf("Added SmartNIC: %s to ETCD. Metadata is %q\n",
				smartNIC, resp)
		}
		// Create the deployment directory for each smartnic.
		resp, err = keysAPI.Set(context.Background(),
			fmt.Sprintf("/deployments/smartnic/%s", smartNIC),
			"", &opts)
		if err != nil {
			log.Fatal(err)
		} else {
			// print common key info
			log.Printf("Added SmartNIC %s Deployments directory to ETCD. "+
				"Metadata is %q\n", smartNIC, resp)
		}
	}
}

func main() {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	functionNamespace := "default"

	if namespace, exists := os.LookupEnv("function_namespace"); exists {
		functionNamespace = namespace
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	readConfig := types.ReadConfig{}
	osEnv := types.OsEnv{}
	cfg := readConfig.Read(osEnv)
	keysAPI := handlers.CreateEtcdClient(etcdMasterIP, etcdPort)
	initializeEtcd(keysAPI)

	log.Printf("HTTP Read Timeout: %s\n", cfg.ReadTimeout)
	log.Printf("HTTP Write Timeout: %s\n", cfg.WriteTimeout)

	deployConfig := &handlers.DeployHandlerConfig{
		HTTPProbe: cfg.HTTPProbe,
		FunctionReadinessProbeConfig: &handlers.FunctionProbeConfig{
			InitialDelaySeconds: int32(cfg.ReadinessProbeInitialDelaySeconds),
			TimeoutSeconds:      int32(cfg.ReadinessProbeTimeoutSeconds),
			PeriodSeconds:       int32(cfg.ReadinessProbePeriodSeconds),
		},
		FunctionLivenessProbeConfig: &handlers.FunctionProbeConfig{
			InitialDelaySeconds: int32(cfg.LivenessProbeInitialDelaySeconds),
			TimeoutSeconds:      int32(cfg.LivenessProbeTimeoutSeconds),
			PeriodSeconds:       int32(cfg.LivenessProbePeriodSeconds),
		},
		ImagePullPolicy: cfg.ImagePullPolicy,
	}

	bootstrapHandlers := bootTypes.FaaSHandlers{
		FunctionProxy: handlers.MakeProxy(functionNamespace,
			keysAPI,
			cfg.ReadTimeout),
		DeleteHandler: handlers.MakeDeleteHandler(functionNamespace,
			keysAPI,
			clientset),
		DeployHandler: handlers.MakeDeployHandler(functionNamespace,
			keysAPI,
			clientset,
			deployConfig),
		FunctionReader: handlers.MakeFunctionReader(functionNamespace,
			keysAPI,
			clientset),
		ReplicaReader: handlers.MakeReplicaReader(functionNamespace,
			keysAPI,
			clientset),
		ReplicaUpdater: handlers.MakeReplicaUpdater(functionNamespace,
			keysAPI,
			clientset),
		UpdateHandler: handlers.MakeUpdateHandler(functionNamespace,
			keysAPI,
			clientset),
		Health: handlers.MakeHealthHandler(),
		InfoHandler: handlers.MakeInfoHandler(version.BuildVersion(),
			version.GitCommit),
	}

	var port int
	port = cfg.Port

	bootstrapConfig := bootTypes.FaaSConfig{
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		TCPPort:      &port,
		EnableHealth: true,
	}

	bootstrap.Serve(&bootstrapHandlers, &bootstrapConfig)
}
