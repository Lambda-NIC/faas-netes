// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package handlers

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"context"
	"fmt"

	"github.com/gorilla/mux"
	"github.com/Lambda-NIC/faas/gateway/requests"
	"github.com/Lambda-NIC/faas-netes/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"go.etcd.io/etcd/client"
)

// MakeReplicaUpdater updates desired count of replicas
func MakeReplicaUpdater(functionNamespace string, keysAPI client.KeysAPI,
											  clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("Update replicas")

		vars := mux.Vars(r)
		functionName := vars["name"]

		req := types.ScaleServiceRequest{}
		if r.Body != nil {
			defer r.Body.Close()
			bytesIn, _ := ioutil.ReadAll(r.Body)
			marshalErr := json.Unmarshal(bytesIn, &req)
			if marshalErr != nil {
				w.WriteHeader(http.StatusBadRequest)
				msg := "Cannot parse request. Please pass valid JSON."
				w.Write([]byte(msg))
				log.Println(msg, marshalErr)
				return
			}
		}
		if strings.Contains(functionName, "lambdanic") {
			log.Printf("Updating replica for %s\n", functionName)
		} else {
			options := metav1.GetOptions{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "extensions/v1beta1",
				},
			}
			deployment, err := clientset.ExtensionsV1beta1().Deployments(functionNamespace).Get(functionName, options)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Unable to lookup function deployment " + functionName))
				log.Println(err)
				return
			}

			replicas := int32(req.Replicas)
			deployment.Spec.Replicas = &replicas
			_, err = clientset.ExtensionsV1beta1().Deployments(functionNamespace).Update(deployment)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Unable to update function deployment " + functionName))
				log.Println(err)
				return
			}
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

// MakeReplicaReader reads the amount of replicas for a deployment
func MakeReplicaReader(functionNamespace string,
											 keysAPI client.KeysAPI,
											 clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("Read replicas")

		vars := mux.Vars(r)
		functionName := vars["name"]
		var function requests.Function

		// LambdaNIC: create dummy replica values.
		// TODO: Fix this.
		if strings.Contains(functionName, "lambdanic") {
			resp, err := keysAPI.Get(context.Background(),
															 fmt.Sprintf("/functions/%s", functionName),
															 nil)
			if err == nil {
				function = requests.Function{
					Name:              functionName,
					Replicas:          4,
					Image:             "smartnic",
					AvailableReplicas: uint64(4),
					InvocationCount:   0,
					Labels:            nil,
					Annotations: 	   	 nil,
				}
				fmt.Printf("Replica Read Key: %q, Value: %q\n",
									 resp.Node.Key,
									 resp.Node.Value)
			}
		} else {
			function, err := getService(functionNamespace, functionName, clientset)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if function == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}

		functionBytes, _ := json.Marshal(function)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(functionBytes)
	}
}
