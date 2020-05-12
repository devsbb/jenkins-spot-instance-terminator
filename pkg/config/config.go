package config

import (
	"flag"
	"fmt"
	"os"
)

const (
	// EC2 Instance Metadata is configurable mainly for testing purposes
	instanceMetadataURLConfigKey = "INSTANCE_METADATA_URL"
	defaultInstanceMetadataURL   = "http://169.254.169.254"
	jenkinsMasterURLKey          = "JENKINS_MASTER_URL"
	jenkinsMasterAPIUserKey      = "JENKINS_MASTER_API_USER"
	jenkinsMasterAPITokenKey     = "JENKINS_MASTER_API_TOKEN"
)

type Config struct {
	MetadataURL           string
	JenkinsMasterURL      string
	JenkinsMasterAPIUser  string
	JenkinsMasterAPIToken string
}

func ParseCliConfig() (*Config, error) {
	config := Config{}

	flag.StringVar(&config.MetadataURL, "metadata-url", getEnv(instanceMetadataURLConfigKey, defaultInstanceMetadataURL), "The URL of EC2 instance metadata. This shouldn't need to be changed unless you are testing.")
	flag.StringVar(&config.JenkinsMasterURL, "jenkins-master-url", getEnv(jenkinsMasterURLKey, ""), "The URL of Jenkins master")
	flag.StringVar(&config.JenkinsMasterAPIUser, "jenkins-master-api-user", getEnv(jenkinsMasterAPIUserKey, "admin"), "The API user of Jenkins master")
	flag.StringVar(&config.JenkinsMasterAPIToken, "jenkins-master-api-token", getEnv(jenkinsMasterAPITokenKey, ""), "The API token of Jenkins master")
	flag.Parse()

	if config.JenkinsMasterURL == "" {
		return nil, fmt.Errorf("jenkins master url is required")
	}

	if config.JenkinsMasterAPIUser == "" {
		return nil, fmt.Errorf("jenkins master api user is required")
	}

	if config.JenkinsMasterAPIToken == "" {
		return nil, fmt.Errorf("jenkins master api token is required")
	}

	return &config, nil
}

func getEnv(name string, fallback string) string {
	value, ok := os.LookupEnv(name)
	if ok {
		return value
	}
	return fallback
}
