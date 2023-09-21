package main

import (
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/sun-asterisk-research/keda-redis-scaler/externalscaler"
)

type ScalerMetadata struct {
	fullName        string
	Address         string `mapstructure:"address"`
	Host            string `mapstructure:"host"`
	Port            string `mapstructure:"port"`
	EnableTLS       bool   `mapstructure:"enableTLS"`
	UnsafeSSL       bool   `mapstructure:"unsafeSSL"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	Database        int    `mapstructure:"database"`
	Script          string `mapstructure:"script"`
	Keys            string `mapstructure:"keys"`
	Args            string `mapstructure:"args"`
	MetricName      string `mapstructure:"metricName"`
	ActivationValue int64  `mapstructure:"activationValue"`
	TargetValue     int64  `mapstructure:"targetValue"`
}

func parseMetadata(scaledObject *externalscaler.ScaledObjectRef) (ScalerMetadata, error) {
	fullName := fmt.Sprintf("%s/%s", scaledObject.Namespace, scaledObject.Name)

	metadata := ScalerMetadata{
		fullName:    fullName,
		Port:        "6379",
		MetricName:  strings.ReplaceAll(fmt.Sprintf("redis-%s", fullName), "/", "-"),
		TargetValue: 5,
	}

	if err := mapstructure.WeakDecode(scaledObject.ScalerMetadata, &metadata); err != nil {
		return metadata, err
	}

	if metadata.Script == "" {
		return metadata, fmt.Errorf("script is required")
	}

	if metadata.TargetValue == 0 {
		metadata.TargetValue = 5
	}

	return metadata, nil
}
