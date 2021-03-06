package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/gorilla/mux"
)

// ScaleServiceRequest request to scale a function
type ScaleServiceRequest struct {
	ServiceName string `json:"serviceName"`
	Replicas    uint64 `json:"replicas"`
}

// ReplicaUpdater updates a function
func ReplicaUpdater(c *client.Client) http.HandlerFunc {
	serviceQuery := NewSwarmServiceQuery(c)

	return func(w http.ResponseWriter, r *http.Request) {

		vars := mux.Vars(r)
		functionName := vars["name"]

		log.Printf("ReplicaUpdater - updating function: %s\n", functionName)

		req := ScaleServiceRequest{}

		if r.Body != nil {
			defer r.Body.Close()

			bytesIn, _ := ioutil.ReadAll(r.Body)
			marshalErr := json.Unmarshal(bytesIn, &req)
			if marshalErr != nil {
				msg := "Cannot parse request. Please pass valid JSON."

				log.Println(msg, marshalErr)

				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(msg))
				return
			}
		}

		log.Printf("Scaling %s to %d replicas", functionName, req.Replicas)

		postStartTs, postEndTs, scaleErr := scaleService(functionName, req.Replicas, serviceQuery)

		log.Printf(fmt.Sprintf("Adding Headers in response: %d %d", postStartTs.UTC().UnixNano(), postEndTs.UTC().UnixNano()))
		w.Header().Add("X-Scale-Post-Send-Time", fmt.Sprintf("%d", postStartTs.UTC().UnixNano()))
		w.Header().Add("X-Scale-Post-Response-Time", fmt.Sprintf("%d", postEndTs.UTC().UnixNano()))
		
		if scaleErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(scaleErr.Error()))
			log.Println(scaleErr.Error())
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func scaleService(serviceName string, newReplicas uint64, service ServiceQuery) (time.Time, time.Time, error) {
	var postStartTs time.Time
	var postEndTs time.Time
	var err error

	if len(serviceName) > 0 {
		startTs, endTs, updateErr := service.SetReplicas(serviceName, newReplicas)
		if updateErr != nil {
			err = updateErr
		}
		postStartTs = startTs
		postEndTs = endTs
	}

	return postStartTs, postEndTs, err
}

// DefaultMaxReplicas is the amount of replicas a service will auto-scale up to.
const DefaultMaxReplicas = 20

// MinScaleLabel label indicating min scale for a function
const MinScaleLabel = "com.openfaas.scale.min"

// MaxScaleLabel label indicating max scale for a function
const MaxScaleLabel = "com.openfaas.scale.max"

// ServiceQuery provides interface for replica querying/setting
type ServiceQuery interface {
	GetReplicas(service string) (currentReplicas uint64, maxReplicas uint64, minReplicas uint64, err error)
	SetReplicas(service string, count uint64) (postStartTs time.Time, postEndTs time.Time, err error)
}

// NewSwarmServiceQuery create new Docker Swarm implementation
func NewSwarmServiceQuery(c *client.Client) ServiceQuery {
	return SwarmServiceQuery{
		c: c,
	}
}

// SwarmServiceQuery implementation for Docker Swarm
type SwarmServiceQuery struct {
	c *client.Client
}

// GetReplicas replica count for function
func (s SwarmServiceQuery) GetReplicas(serviceName string) (uint64, uint64, uint64, error) {
	var err error
	var currentReplicas uint64

	maxReplicas := uint64(DefaultMaxReplicas)
	minReplicas := uint64(1)

	opts := types.ServiceInspectOptions{
		InsertDefaults: true,
	}

	service, _, err := s.c.ServiceInspectWithRaw(context.Background(), serviceName, opts)

	if err == nil {
		currentReplicas = *service.Spec.Mode.Replicated.Replicas

		minScale := service.Spec.Annotations.Labels[MinScaleLabel]
		maxScale := service.Spec.Annotations.Labels[MaxScaleLabel]

		if len(maxScale) > 0 {
			labelValue, err := strconv.Atoi(maxScale)
			if err != nil {
				log.Printf("Bad replica count: %s, should be uint", maxScale)
			} else {
				maxReplicas = uint64(labelValue)
			}
		}

		if len(minScale) > 0 {
			labelValue, err := strconv.Atoi(maxScale)
			if err != nil {
				log.Printf("Bad replica count: %s, should be uint", minScale)
			} else {
				minReplicas = uint64(labelValue)
			}
		}
	}

	return currentReplicas, maxReplicas, minReplicas, err
}

// SetReplicas update the replica count
func (s SwarmServiceQuery) SetReplicas(serviceName string, count uint64) (time.Time, time.Time, error) {
	opts := types.ServiceInspectOptions{
		InsertDefaults: true,
	}

	var postStartTs time.Time
	var postEndTs time.Time

	service, _, err := s.c.ServiceInspectWithRaw(context.Background(), serviceName, opts)
	if err == nil {

		service.Spec.Mode.Replicated.Replicas = &count
		updateOpts := types.ServiceUpdateOptions{}
		updateOpts.RegistryAuthFrom = types.RegistryAuthFromSpec

		postStartTs = time.Now()
		_, updateErr := s.c.ServiceUpdate(context.Background(), service.ID, service.Version, service.Spec, updateOpts)
		postEndTs = time.Now()
		if updateErr != nil {
			err = updateErr
		}
	}

	return postStartTs, postEndTs, err
}
