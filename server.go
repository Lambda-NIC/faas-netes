// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package main

import (
	"log"
	"os"
	"time"
	"fmt"
	"context"

	"github.com/Lambda-NIC/faas-netes/handlers"
	"github.com/Lambda-NIC/faas-netes/types"
	"github.com/Lambda-NIC/faas-netes/version"
	"github.com/Lambda-NIC/faas-provider"
	bootTypes "github.com/Lambda-NIC/faas-provider/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"go.etcd.io/etcd/client"
)

// LambdaNIC: Create etcd connection for saving distributed values.
const etcdMasterIP string = "30.30.30.105"
const etcdPort string = "2379"

// LambdaNIC: List of SmartNICs to use and how many deployments are there.
var smartNICs = []string{"20.20.20.101", "20.20.20.102",
												 "20.20.20.103", "20.20.20.104"}

func createEtcdClient() client.KeysAPI {
	cfg := client.Config{
		Endpoints: []string{fmt.Sprintf("https://%s:%s", etcdMasterIP, etcdPort)},
		Transport: client.DefaultTransport,
		// set timeout per request to fail fast when
		// the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}
	c, err := client.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	kapi := client.NewKeysAPI(c)
	return kapi
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
	keysAPI := createEtcdClient()

	opts := client.SetOptions{Dir: true}
	resp, err := keysAPI.Set(context.Background(),
																	 "/smartnics",
																	 "", &opts)
	if err != nil {
		log.Fatal(err)
	} else {
		// print common key info
		log.Printf("Added SmartNIC directory to ETCD. Metadata is %q\n",
							 resp)
	}
	resp, err = keysAPI.Set(context.Background(),
													"/deployments",
													"", &opts)
	if err != nil {
		log.Fatal(err)
	} else {
		// print common key info
		log.Printf("Added Deployments directory to ETCD. Metadata is %q\n",
							 resp)
	}
	resp, err = keysAPI.Set(context.Background(),
													"/functions",
													"", &opts)
	if err != nil {
		log.Fatal(err)
	} else {
		// print common key info
		log.Printf("Added Functions directory to ETCD. Metadata is %q\n",
							 resp)
	}

	for i, smartNIC := range smartNICs {
		resp, err = keysAPI.Set(context.Background(),
									 				 fmt.Sprintf("/smartnics/%d", i),
									 			 	 smartNIC, nil)
		if err != nil {
			log.Fatal(err)
		} else {
			// print common key info
			log.Printf("Added SmartNIC Server: %s to ETCD. Metadata is %q\n",
								 smartNIC, resp)
		}
		resp, err = keysAPI.Set(context.Background(),
														"/deployments/smartnic%d",
														"", &opts)
		if err != nil {
			log.Fatal(err)
		} else {
			// print common key info
			log.Printf("Added SmartNIC %d Deployments directory to ETCD." +
								 "Metadata is %q\n",
								 i, resp)
		}
	}
	_, err = keysAPI.Set(context.Background(),"numServers",
											 string(len(smartNICs)), nil)
	if err != nil {
		log.Fatal(err)
	} else {
		log.Printf("Set the number of SmartNICs as %d in ETCD", len(smartNICs))
	}

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
		FunctionProxy:  handlers.MakeProxy(functionNamespace,
																			 keysAPI,
																			 cfg.ReadTimeout),
		DeleteHandler:  handlers.MakeDeleteHandler(functionNamespace,
																							 keysAPI,
																							 &smartNICs,
																							 clientset),
		DeployHandler:  handlers.MakeDeployHandler(functionNamespace,
																							 keysAPI,
																							 &smartNICs,
																							 clientset,
																							 deployConfig),
		FunctionReader: handlers.MakeFunctionReader(functionNamespace,
																								keysAPI,
																								clientset),
		ReplicaReader:  handlers.MakeReplicaReader(functionNamespace,
																							 keysAPI,
																							 clientset),
		ReplicaUpdater: handlers.MakeReplicaUpdater(functionNamespace,
																								keysAPI,
																							  clientset),
		UpdateHandler:  handlers.MakeUpdateHandler(functionNamespace,
																							 keysAPI,
																							 clientset),
		Health:         handlers.MakeHealthHandler(),
		InfoHandler:    handlers.MakeInfoHandler(version.BuildVersion(),
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
