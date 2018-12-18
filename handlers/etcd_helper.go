package handlers

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.etcd.io/etcd/client"
)

// CreateEtcdClient creates a client for ETCD deployment
func CreateEtcdClient(etcdMasterIP string, etcdPort string) client.KeysAPI {
	cfg := client.Config{
		Endpoints: []string{fmt.Sprintf("http://%s:%s", etcdMasterIP, etcdPort)},
		Transport: client.DefaultTransport,
		// set timeout per request to fail fast when
		// the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}
	c, err := client.New(cfg)
	if err != nil {
		log.Fatal("Could not connect to ETCD: " + err.Error())
	}
	kapi := client.NewKeysAPI(c)
	return kapi
}

// CreateDepKey creates a key for deployment
func CreateDepKey(smartNIC string, funcName string) string {
	return fmt.Sprintf("/deployments/smartnic/%s/%s", smartNIC,
		funcName)
}

// CreateFuncKey creates a key for function
func CreateFuncKey(funcName string) string {
	return fmt.Sprintf("/functions/%s", funcName)
}

// EtcdFunctionCreate creates a function in etcd.
func EtcdFunctionCreate(keysAPI client.KeysAPI,
	funcName string) error {
	uid := fmt.Sprintf("%d", time.Now().Nanosecond())
	funcKey := CreateFuncKey(funcName)
	resp, err := keysAPI.Set(context.Background(), funcKey, uid, nil)
	if err != nil {
		return err
	}
	log.Printf("Added func: %s id: %s to ETCD. Metadata: %q\n",
		funcName, uid, resp)
	smartNICs, err := GetSmartNICS(keysAPI)
	if err != nil {
		return err
	}
	numTries := 0
	for {
		randIdx := rand.Intn(len(smartNICs))
		smartNIC := smartNICs[randIdx]
		var depKey = CreateDepKey(smartNIC, funcName)
		resp, err = keysAPI.Set(context.Background(), depKey, "1", nil)
		if err != nil {
			numTries++
			if numTries > 10 {
				_, _ = keysAPI.Delete(context.Background(), funcKey, nil)
				return err
			}
			continue
		}
		log.Printf("Added a Dep: %s to ECTD. Metadata: %q\n", depKey, resp)
		log.Printf("Created SmartNIC service - %s at %s\n", funcName, smartNIC)
		break
	}
	return nil
}

// EtcdFunctionDelete deletes the function with function name
func EtcdFunctionDelete(keysAPI client.KeysAPI, funcName string) error {
	smartNICs, err := GetSmartNICS(keysAPI)
	if err != nil {
		return err
	}
	for _, smartNIC := range smartNICs {
		log.Println("Deleting deployment.")
		_, err = keysAPI.Delete(context.Background(),
			CreateDepKey(smartNIC, funcName), nil)
		if err != nil {
			log.Printf("Couldn't find deployment at server %s\n", smartNIC)
		} else {
			log.Printf("Deleted %s in server %s\n", funcName, smartNIC)
		}
	}
	_, err = keysAPI.Delete(context.Background(), CreateFuncKey(funcName), nil)
	if err != nil {
		return err
	}
	log.Printf("Deleted Function: %s\n", funcName)
	return nil
}

// EtcdFunctionExists checks if the function exists in etcd.
func EtcdFunctionExists(keysAPI client.KeysAPI, functionName string) bool {
	_, err := keysAPI.Get(context.Background(),
		fmt.Sprintf("/functions/%s", functionName),
		nil)
	// Did not find the function.
	return err == nil
}

// GetSmartNICS returns the list of SmartNICs from ETCD.
func GetSmartNICS(keysAPI client.KeysAPI) ([]string, error) {
	resp, err := keysAPI.Get(context.Background(), "/smartnics", nil)
	// No smartnics found in deployment.
	if err != nil {
		log.Println("Could not retrieve SmartNICs")
		return nil, err
	}
	// print directory keys
	sort.Sort(resp.Node.Nodes)
	smartNICs := make([]string, len(resp.Node.Nodes))
	for _, n := range resp.Node.Nodes {
		smartNIC := strings.Split(n.Key, "/")[2]
		smartNICs = append(smartNICs, smartNIC)
	}
	return smartNICs, nil
}

// GetFunctions returns the list of functions
func GetFunctions(keysAPI client.KeysAPI) ([]string, error) {
	resp, err := keysAPI.Get(context.Background(), "/functions", nil)
	if err != nil {
		return nil, err
	}
	var functions []string
	// print directory keys
	sort.Sort(resp.Node.Nodes)
	for _, n := range resp.Node.Nodes {
		function := strings.Split(n.Key, "/")[2]
		functions = append(functions, function)
	}
	return functions, nil
}

// GetNumDeployments gives the number of deployments for the function.
func GetNumDeployments(keysAPI client.KeysAPI,
	funcName string) (uint64, error) {

	var numReplicas uint64
	smartNICs, err := GetSmartNICS(keysAPI)
	if err != nil {
		return 0, err
	}
	log.Printf("Got %d SmartNICS.\n", len(smartNICs))
	for _, smartNIC := range smartNICs {
		depVal, depErr := keysAPI.Get(context.Background(),
			CreateDepKey(smartNIC, funcName), nil)
		log.Printf("Got %s deps for %s SmartNICS.\n", depVal.Node.Value, smartNIC)
		if depErr != nil {
			// Deployment doesn't exist
			continue
		} else {
			numDep, numDepErr := strconv.ParseUint(depVal.Node.Value, 10, 64)
			if numDepErr != nil {
				numReplicas += numDep
			}
		}
	}
	return numReplicas, nil
}
