package jenkins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-node-termination-handler/pkg/ec2metadata"
	"github.com/rs/zerolog/log"
)

const (
	toggleOfflineURL    = "/computer/%s/toggleOffline"
	slaveInformationURL = "/computer/%s/api/json"
	methodPost          = "POST"
	methodGet           = "GET"
)

type Jenkins struct {
	MasterBaseAPI   string
	MasterUser      string
	MasterToken     string
	MachineMetadata ec2metadata.NodeMetadata
	httpClient      *http.Client
}

func New(masterBaseAPI string, masterUser string, masterToken string, machineMetadata ec2metadata.NodeMetadata) *Jenkins {
	return &Jenkins{
		MasterBaseAPI:   masterBaseAPI,
		MasterUser:      masterUser,
		MasterToken:     masterToken,
		MachineMetadata: machineMetadata,
		httpClient:      &http.Client{},
	}
}

func (jenkins *Jenkins) MarkCurrentSlaveAsOffline() error {
	log.Printf("Marking current slave %v as offline", jenkins.getSlaveName())
	isOnline, err := jenkins.isSlaveOnline()
	if err != nil {
		log.Error().Err(err).Msg("failed to check if the node is offline")
		return err
	}
	if isOnline {
		return jenkins.toggleSlaveState()
	}
	log.Printf("Slave was already offline %v", jenkins.getSlaveName())
	time.Sleep(5_000)
	return nil
}

func (jenkins *Jenkins) MarkCurrentSlaveAsOnline() error {
	log.Printf("Marking current slave %v was online", jenkins.getSlaveName())
	return jenkins.toggleSlaveState()
}

func (jenkins *Jenkins) toggleSlaveState() error {
	request := jenkins.newRequest(methodPost, jenkins.getToggleOfflineURL())
	response, err := jenkins.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		if event := log.Debug(); event.Enabled() {
			buffer := new(bytes.Buffer)
			_, _ = buffer.ReadFrom(response.Body)
			event.Msgf("jenkins response:\n %v", buffer.String())
		}
		return fmt.Errorf("jenkins returned an invalid status code (%v) while toggling slave's state", response.StatusCode)
	}
	return nil
}

func (jenkins *Jenkins) isSlaveOnline() (bool, error) {
	request := jenkins.newRequest(methodGet, jenkins.getSlaveInfoURL())
	response, err := jenkins.httpClient.Do(request)
	if err != nil {
		return false, err
	}
	if response.StatusCode != 200 {
		return false, fmt.Errorf("error checking in slave %v is online, jenkins returned %v status code", jenkins.getSlaveName(), response.StatusCode)
	}
	defer response.Body.Close()
	var ss SlaveStatus
	err = json.NewDecoder(response.Body).Decode(&ss)
	if err != nil {
		return false, fmt.Errorf("failed to parse slave status json output %v", err)
	}

	return !ss.Offline && !ss.TemporarilyOffline, nil
}

func (jenkins *Jenkins) getSlaveName() string {
	return jenkins.MachineMetadata.InstanceID
}

func (jenkins *Jenkins) getToggleOfflineURL() string {
	suffix := fmt.Sprintf(toggleOfflineURL, jenkins.getSlaveName())
	return fmt.Sprintf("%s%s", jenkins.MasterBaseAPI, suffix)
}

func (jenkins *Jenkins) getSlaveInfoURL() string {
	suffix := fmt.Sprintf(slaveInformationURL, jenkins.getSlaveName())
	return fmt.Sprintf("%s%s", jenkins.MasterBaseAPI, suffix)
}

func (jenkins *Jenkins) newRequest(method string, requestUrl string) *http.Request {
	request, err := http.NewRequest(method, requestUrl, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create a post request for jenkins")
	}
	request.SetBasicAuth(jenkins.MasterUser, jenkins.MasterToken)
	request.Header.Add("content-type", "application/json")
	return request
}
