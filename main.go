package main

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sun-asterisk-research/keda-redis-scaler/externalscaler"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

type RedisScaler struct {
	connections *redisConnectionManager
	logger      *zap.SugaredLogger
	externalscaler.UnimplementedExternalScalerServer
}

func (rs *RedisScaler) IsActive(ctx context.Context, scaledObject *externalscaler.ScaledObjectRef) (*externalscaler.IsActiveResponse, error) {
	metadata, err := parseMetadata(scaledObject)
	logger := rs.logger.With("method", "IsActive", "scaledObject", metadata.fullName)
	if err != nil {
		logger.Error("Error parsing metadata: ", err)
		return nil, err
	}

	getMetricsResp, err := rs.GetMetrics(ctx, &externalscaler.GetMetricsRequest{
		ScaledObjectRef: scaledObject,
	})

	if err != nil {
		logger.Error("Get metrics failed", err)
		return nil, err
	}

	metricValues := getMetricsResp.GetMetricValues()
	if len(metricValues) != 1 {
		err := errors.New("GetMetrics must return exactly one value")
		logger.Error("invalid GetMetricsResponse: ", err)
		return nil, err
	}

	return &externalscaler.IsActiveResponse{
		Result: metricValues[0].MetricValue > metadata.ActivationValue,
	}, nil
}

func (rs *RedisScaler) StreamIsActive(scaledObject *externalscaler.ScaledObjectRef, server externalscaler.ExternalScaler_StreamIsActiveServer) error {
	metadata, err := parseMetadata(scaledObject)
	logger := rs.logger.With("method", "StreamIsActive", "scaledObject", metadata.fullName)
	if err != nil {
		logger.Error("Error parsing metadata: ", err)
		return err
	}

	// Just tell the server to call IsActive, may be every 5 seconds?
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-server.Context().Done():
			return nil
		case <-ticker.C:
			resp, err := rs.IsActive(server.Context(), scaledObject)
			if err != nil {
				logger.Error("Error getting active status in stream: ", err)
				return err
			}

			if err = server.Send(resp); err != nil {
				logger.Error("Error sending the active result in stream: ", err)
				return err
			}
		}
	}
}

func (rs *RedisScaler) GetMetricSpec(ctx context.Context, scaledObject *externalscaler.ScaledObjectRef) (*externalscaler.GetMetricSpecResponse, error) {
	metadata, err := parseMetadata(scaledObject)
	if err != nil {
		rs.logger.With("method", "GetMetricSpec", "scaledObject", metadata.fullName).Error("Error parsing metadata: ", err)
		return nil, err
	}

	return &externalscaler.GetMetricSpecResponse{
		MetricSpecs: []*externalscaler.MetricSpec{
			{
				MetricName: metadata.MetricName,
				TargetSize: metadata.TargetValue,
			},
		},
	}, nil
}

func (rs *RedisScaler) GetMetrics(ctx context.Context, req *externalscaler.GetMetricsRequest) (*externalscaler.GetMetricsResponse, error) {
	metadata, err := parseMetadata(req.ScaledObjectRef)
	logger := rs.logger.With("method", "GetMetrics", "scaledObject", metadata.fullName)
	if err != nil {
		logger.Error("Error parsing metadata: ", err)
		return nil, err
	}

	rdb, err := rs.connections.getRedisClient(metadata)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	var keys, args []string

	if metadata.Keys != "" {
		if err := yaml.Unmarshal([]byte(metadata.Keys), &keys); err != nil {
			logger.Error("Error parsing keys: ", err)
			return nil, err
		}
	}

	if metadata.Args != "" {
		if err := yaml.Unmarshal([]byte(metadata.Args), &args); err != nil {
			logger.Error("Error parsing args: ", err)
			return nil, err
		}
	}

	num, err := redis.NewScript(metadata.Script).Run(ctx, rdb, keys, args).Int()
	if err != nil {
		logger.Error("Error executing Lua script: ", err)
		return nil, err
	}

	logger.With("metricName", req.MetricName, "metricValue", num).Debug("Got metric value")

	return &externalscaler.GetMetricsResponse{
		MetricValues: []*externalscaler.MetricValue{
			{
				MetricName:  req.MetricName,
				MetricValue: int64(num),
			},
		},
	}, nil
}

func (rs *RedisScaler) Close() {
	rs.connections.cleanup()
}

func NewRedisScaler(logger *zap.Logger) *RedisScaler {
	connections := NewRedisConnectionManager(logger)

	return &RedisScaler{
		connections: connections,
		logger:      logger.Sugar(),
	}
}

func newLogger(conf Config) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(conf.LogLevel)
	if err != nil {
		return nil, err
	}

	zapConfig := zap.NewProductionConfig()
	zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapConfig.Level = zap.NewAtomicLevelAt(level)

	if level == zap.DebugLevel {
		zapConfig.Development = true
		zapConfig.Encoding = "console"
	} else {
		zapConfig.DisableStacktrace = true
	}

	return zapConfig.Build()
}

func main() {
	logger, err := newLogger(conf)
	if err != nil {
		panic(err)
	}

	address := net.JoinHostPort(conf.Host, conf.Port)

	server := grpc.NewServer()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		logger.Fatal(err.Error())
	}

	logger.Info("Starting server on " + address)

	scaler := NewRedisScaler(logger)
	defer scaler.Close()

	externalscaler.RegisterExternalScalerServer(server, scaler)

	if err = server.Serve(listener); err != nil {
		logger.Fatal(err.Error())
	}
}
