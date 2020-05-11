// File based on https://github.com/aws/aws-node-termination-handler
package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	nthConfigModule "github.com/aws/aws-node-termination-handler/pkg/config"
	"github.com/aws/aws-node-termination-handler/pkg/ec2metadata"
	"github.com/aws/aws-node-termination-handler/pkg/interruptionevent"
	"github.com/aws/aws-node-termination-handler/pkg/interruptioneventstore"
	"github.com/devsbb/jenkins-spot-instance-terminator/pkg/config"
	"github.com/devsbb/jenkins-spot-instance-terminator/pkg/jenkins"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	scheduledMaintenance = "Scheduled Maintenance"
	spotITN              = "Spot ITN"
)

type monitorFunc func(chan<- interruptionevent.InterruptionEvent, chan<- interruptionevent.InterruptionEvent, *ec2metadata.Service) error

func main() {
	// Zerolog uses json formatting by default, so change that to a human-readable format instead
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339, NoColor: true})

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	defer signal.Stop(signalChan)

	parsedConfig, err := config.ParseCliConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse cli args,")
	}

	imds := ec2metadata.New(parsedConfig.MetadataURL, 30)

	nthConfig := getNthConfig(parsedConfig)
	interruptionEventStore := interruptioneventstore.New(nthConfig)
	nodeMetadata := imds.GetNodeMetadata()
	jenkinsMaster := jenkins.New(parsedConfig.JenkinsMasterURL, parsedConfig.JenkinsMasterAPIUser, parsedConfig.JenkinsMasterAPIToken, nodeMetadata)

	interruptionChan := make(chan interruptionevent.InterruptionEvent)
	defer close(interruptionChan)
	cancelChan := make(chan interruptionevent.InterruptionEvent)
	defer close(cancelChan)

	monitoringFns := map[string]monitorFunc{}
	monitoringFns[spotITN] = interruptionevent.MonitorForSpotITNEvents
	monitoringFns[scheduledMaintenance] = interruptionevent.MonitorForScheduledEvents

	for eventType, fn := range monitoringFns {
		go func(monitorFn monitorFunc, eventType string) {
			log.Printf("Started monitoring for %s events", eventType)
			for range time.Tick(time.Second * 2) {
				err := monitorFn(interruptionChan, cancelChan, imds)
				if err != nil {
					log.Printf("There was a problem monitoring for %s events: %v", eventType, err)
				}
			}
		}(fn, eventType)
	}

	go watchForInterruptionEvents(interruptionChan, interruptionEventStore, nodeMetadata)
	log.Print("Started watching for interruption events")

	go watchForCancellationEvents(cancelChan, interruptionEventStore, jenkinsMaster, nodeMetadata)
	log.Print("Started watching for event cancellations")

	for range time.NewTicker(1 * time.Second).C {
		select {
		case <-signalChan:
			// Exit interruption loop if a SIGTERM is received or the channel is closed
			break
		default:
			markSlaveAsOffline(interruptionEventStore, jenkinsMaster, nodeMetadata)
		}
	}
	log.Print("AWS Node Termination Handler is shutting down")
}

func getNthConfig(parsedConfig *config.Config) nthConfigModule.Config {
	return nthConfigModule.Config{
		DryRun:                         false,
		NodeName:                       "",
		MetadataURL:                    parsedConfig.MetadataURL,
		IgnoreDaemonSets:               false,
		DeleteLocalData:                false,
		KubernetesServiceHost:          "",
		KubernetesServicePort:          "",
		PodTerminationGracePeriod:      0,
		NodeTerminationGracePeriod:     60 * 5,
		WebhookURL:                     "",
		WebhookHeaders:                 "",
		WebhookTemplate:                "",
		EnableScheduledEventDraining:   true,
		EnableSpotInterruptionDraining: true,
		MetadataTries:                  3,
		CordonOnly:                     false,
		JsonLogging:                    false,
	}
}

func watchForInterruptionEvents(interruptionChan <-chan interruptionevent.InterruptionEvent, interruptionEventStore *interruptioneventstore.Store, nodeMetadata ec2metadata.NodeMetadata) {
	for {
		interruptionEvent := <-interruptionChan
		log.Printf("Got interruption event from channel %+v %+v", nodeMetadata, interruptionEvent)
		interruptionEventStore.AddInterruptionEvent(&interruptionEvent)
	}
}

func watchForCancellationEvents(cancelChan <-chan interruptionevent.InterruptionEvent, interruptionEventStore *interruptioneventstore.Store, jenkinsMaster *jenkins.Jenkins, nodeMetadata ec2metadata.NodeMetadata) {
	for {
		interruptionEvent := <-cancelChan
		log.Printf("Got cancel event from channel %+v %+v", nodeMetadata, interruptionEvent)
		interruptionEventStore.CancelInterruptionEvent(interruptionEvent.EventID)
		if interruptionEventStore.ShouldUncordonNode() {
			log.Print("Marking the current slave as online due to a cancellation event")
			err := jenkinsMaster.MarkCurrentSlaveAsOnline()
			if err != nil {
				log.Printf("Marking the current slave as online failed: %v", err)
			}
		} else {
			log.Print("Another interruption event is active, not marking the current slave as online")
		}
	}
}

func markSlaveAsOffline(interruptionEventStore *interruptioneventstore.Store, jenkinsMaster *jenkins.Jenkins, nodeMetadata ec2metadata.NodeMetadata) {
	if _, ok := interruptionEventStore.GetActiveEvent(); ok {
		err := jenkinsMaster.MarkCurrentSlaveAsOffline()
		if err != nil {
			log.Printf("There was a problem while trying mark the current slave %s as offline: %v", nodeMetadata.InstanceID, err)
			os.Exit(1)
		}
		log.Printf("Current slave %s successfully marked as offline.", nodeMetadata.InstanceID)
		interruptionEventStore.MarkAllAsDrained()
	}
}
