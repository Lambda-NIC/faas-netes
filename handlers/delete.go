// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package handlers

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"log"
	"fmt"
	"context"
	"strconv"

	"github.com/Lambda-NIC/faas/gateway/requests"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"go.etcd.io/etcd/client"
)

// MakeDeleteHandler delete a function
func MakeDeleteHandler(functionNamespace string,
											 keysAPI client.KeysAPI,
											 smartNICs *[]string,
											 clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		body, _ := ioutil.ReadAll(r.Body)

		request := requests.DeleteFunctionRequest{}
		err := json.Unmarshal(body, &request)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if len(request.FunctionName) == 0 {
			w.WriteHeader(http.StatusBadRequest)
		}

		// LambdaNIC: Delete scheme for lambdanic
		if strings.Contains(request.FunctionName, "lambdanic") {
			log.Printf("Got request to delete: %s", request.FunctionName)
			splitStr := strings.Split(request.FunctionName, "/")
			_, functionName := splitStr[0], splitStr[1]

			node, err := keysAPI.Get(context.Background(), "numServers", nil)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Error Deleting Function:" + functionName))
				log.Println("No numServers key: " + err.Error())
				return
			}
			numServers, err := strconv.Atoi(node.Node.Value)
			if err != nil {
				log.Fatal("Invalid numServer value in etcd db: " + node.Node.Value)
			}
			log.Printf("%d servers available\n", numServers)


			// Check if this service exists
			var jobID string = fmt.Sprintf("/functions/%s", functionName)
			log.Printf("Deleting function with key: %s\n", jobID)
			_, err = keysAPI.Delete(context.Background(), jobID, nil)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Error Deleting Function:" + functionName))
				log.Println("Error: " + err.Error())
				return
			}
			log.Printf("Deleted Function: %s\n", functionName)

			for i := 0; i < numServers; i++ {
				log.Println("Deleting deployment.")
				var depKey string = fmt.Sprintf("/deployments/smartnic%d/%s",
																				 i,
																				 functionName)
				_, err = keysAPI.Delete(context.Background(), depKey, nil)
				if err != nil {
					log.Printf("Couldn't find deployment at server %d\n", i)
				} else {
					log.Printf("Deleted %s in server %d\n", functionName, i)
				}
			}
			log.Println("Deleted SmartNIC service - " + functionName)
			log.Println(string(body))
		} else {
			getOpts := metav1.GetOptions{}

			// This makes sure we don't delete non-labelled deployments
			deployment, findDeployErr := clientset.ExtensionsV1beta1().
				Deployments(functionNamespace).
				Get(request.FunctionName, getOpts)

			if findDeployErr != nil {
				if errors.IsNotFound(findDeployErr) {
					w.WriteHeader(http.StatusNotFound)
				} else {
					w.WriteHeader(http.StatusInternalServerError)
				}

				w.Write([]byte(findDeployErr.Error()))
				return
			}

			if isFunction(deployment) {
				deleteFunction(functionNamespace, clientset, request, w)
			} else {
				w.WriteHeader(http.StatusBadRequest)

				w.Write([]byte("Not a function: " + request.FunctionName))
				return
			}
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func isFunction(deployment *v1beta1.Deployment) bool {
	if deployment != nil {
		if _, found := deployment.Labels["faas_function"]; found {
			return true
		}
	}
	return false
}

func deleteFunction(functionNamespace string, clientset *kubernetes.Clientset, request requests.DeleteFunctionRequest, w http.ResponseWriter) {
	foregroundPolicy := metav1.DeletePropagationForeground
	opts := &metav1.DeleteOptions{PropagationPolicy: &foregroundPolicy}

	if deployErr := clientset.ExtensionsV1beta1().
		Deployments(functionNamespace).
		Delete(request.FunctionName, opts); deployErr != nil {

		if errors.IsNotFound(deployErr) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write([]byte(deployErr.Error()))
		return
	}

	if svcErr := clientset.CoreV1().
		Services(functionNamespace).
		Delete(request.FunctionName, opts); svcErr != nil {

		if errors.IsNotFound(svcErr) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}

		w.Write([]byte(svcErr.Error()))
		return
	}
}
