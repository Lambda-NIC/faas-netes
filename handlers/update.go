package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Lambda-NIC/faas/gateway/requests"
	"go.etcd.io/etcd/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// MakeUpdateHandler update specified function
func MakeUpdateHandler(functionNamespace string,
	keysAPI client.KeysAPI,
	clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		defer r.Body.Close()

		body, _ := ioutil.ReadAll(r.Body)

		request := requests.CreateFunctionRequest{}

		// Make sure the deployment only occurs at nodes with smartnics
		request.Constraints = append(request.Constraints, "smartnic=enabled")

		err := json.Unmarshal(body, &request)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// LambdaNIC: Update a function
		if strings.Contains(request.Service, "lambdanic") {
			// TODO: Need to do this.
			w.Write([]byte("Updated!"))
		} else {
			annotations := buildAnnotations(request)
			if status, err := updateDeploymentSpec(functionNamespace,
				clientset, request,
				annotations); err != nil {
				w.WriteHeader(status)
				w.Write([]byte(err.Error()))
			}

			if status, err := updateService(functionNamespace, clientset,
				request, annotations); err != nil {
				w.WriteHeader(status)
				w.Write([]byte(err.Error()))
			}
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func updateDeploymentSpec(
	functionNamespace string,
	clientset *kubernetes.Clientset,
	request requests.CreateFunctionRequest,
	annotations map[string]string) (httpStatus int, err error) {
	getOpts := metav1.GetOptions{}

	deployment, findDeployErr := clientset.ExtensionsV1beta1().
		Deployments(functionNamespace).
		Get(request.Service, getOpts)

	if findDeployErr != nil {
		return http.StatusNotFound, findDeployErr
	}

	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		deployment.Spec.Template.Spec.Containers[0].Image = request.Image

		// Disabling update support to prevent unexpected mutations of deployed functions,
		// since imagePullPolicy is now configurable. This could be reconsidered later depending
		// on desired behavior, but will need to be updated to take config.
		//deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = v1.PullAlways

		deployment.Spec.Template.Spec.Containers[0].Env = buildEnvVars(&request)

		configureReadOnlyRootFilesystem(request, deployment)

		deployment.Spec.Template.Spec.NodeSelector = createSelector(request.Constraints)

		labels := map[string]string{
			"faas_function": request.Service,
			"uid":           fmt.Sprintf("%d", time.Now().Nanosecond()),
		}

		if request.Labels != nil {
			if min := getMinReplicaCount(*request.Labels); min != nil {
				deployment.Spec.Replicas = min
			}

			for k, v := range *request.Labels {
				labels[k] = v
			}
		}

		deployment.Labels = labels
		deployment.Spec.Template.ObjectMeta.Labels = labels

		deployment.Annotations = annotations
		deployment.Spec.Template.Annotations = annotations
		deployment.Spec.Template.ObjectMeta.Annotations = annotations

		resources, resourceErr := createResources(request)
		if resourceErr != nil {
			return http.StatusBadRequest, resourceErr
		}

		deployment.Spec.Template.Spec.Containers[0].Resources = *resources

		existingSecrets, err := getSecrets(clientset, functionNamespace, request.Secrets)
		if err != nil {
			return http.StatusBadRequest, err
		}

		err = UpdateSecrets(request, deployment, existingSecrets)
		if err != nil {
			log.Println(err)
			return http.StatusBadRequest, err
		}
	}

	if _, updateErr := clientset.ExtensionsV1beta1().
		Deployments(functionNamespace).
		Update(deployment); updateErr != nil {

		return http.StatusInternalServerError, updateErr
	}

	return http.StatusAccepted, nil
}

func updateService(
	functionNamespace string,
	clientset *kubernetes.Clientset,
	request requests.CreateFunctionRequest,
	annotations map[string]string) (httpStatus int, err error) {

	getOpts := metav1.GetOptions{}

	service, findServiceErr := clientset.CoreV1().
		Services(functionNamespace).
		Get(request.Service, getOpts)

	if findServiceErr != nil {
		return http.StatusNotFound, findServiceErr
	}

	service.Annotations = annotations

	if _, updateErr := clientset.CoreV1().
		Services(functionNamespace).
		Update(service); updateErr != nil {

		return http.StatusInternalServerError, updateErr
	}

	return http.StatusAccepted, nil
}
