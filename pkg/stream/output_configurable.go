package stream

import (
	"context"
	"fmt"

	"github.com/justtrackio/gosoline/pkg/cfg"
	"github.com/justtrackio/gosoline/pkg/cloud/aws/sqs"
	"github.com/justtrackio/gosoline/pkg/exec"
	"github.com/justtrackio/gosoline/pkg/log"
)

const (
	OutputTypeFile     = "file"
	OutputTypeInMemory = "inMemory"
	OutputTypeKinesis  = "kinesis"
	OutputTypeMultiple = "multiple"
	OutputTypeNoOp     = "noop"
	OutputTypeRedis    = "redis"
	OutputTypeSns      = "sns"
	OutputTypeSqs      = "sqs"
)

type BaseOutputSettings struct {
	Tracing struct {
		Enabled bool `cfg:"enabled" default:"true"`
	} `cfg:"tracing"`
}

func NewConfigurableOutput(ctx context.Context, config cfg.Config, logger log.Logger, name string) (Output, error) {
	outputFactories := map[string]OutputFactory{
		OutputTypeFile:     newFileOutputFromConfig,
		OutputTypeInMemory: newInMemoryOutputFromConfig,
		OutputTypeKinesis:  newKinesisOutputFromConfig,
		OutputTypeMultiple: NewConfigurableMultiOutput,
		OutputTypeNoOp:     newNoOpOutput,
		OutputTypeRedis:    newRedisListOutputFromConfig,
		OutputTypeSns:      newSnsOutputFromConfig,
		OutputTypeSqs:      newSqsOutputFromConfig,
	}

	key := fmt.Sprintf("%s.type", ConfigurableOutputKey(name))
	typ := config.GetString(key)

	var ok bool
	var err error
	var factory OutputFactory
	var output Output

	if factory, ok = outputFactories[typ]; !ok {
		return nil, fmt.Errorf("invalid output %s of type %s", name, typ)
	}

	if output, err = factory(ctx, config, logger, name); err != nil {
		return nil, fmt.Errorf("can not create output %s: %w", name, err)
	}

	return NewOutputTracer(config, logger, output, name)
}

func newFileOutputFromConfig(_ context.Context, config cfg.Config, logger log.Logger, name string) (Output, error) {
	key := ConfigurableOutputKey(name)
	settings := &FileOutputSettings{}
	config.UnmarshalKey(key, settings)

	return NewFileOutput(config, logger, settings), nil
}

func newInMemoryOutputFromConfig(_ context.Context, _ cfg.Config, _ log.Logger, name string) (Output, error) {
	return ProvideInMemoryOutput(name), nil
}

type kinesisOutputConfiguration struct {
	StreamName string `cfg:"stream_name"`
	Backoff    exec.BackoffSettings
}

func newKinesisOutputFromConfig(_ context.Context, config cfg.Config, logger log.Logger, name string) (Output, error) {
	key := ConfigurableOutputKey(name)
	settings := &kinesisOutputConfiguration{}
	config.UnmarshalKey(key, settings)

	settings.Backoff = exec.ReadBackoffSettings(config)

	return NewKinesisOutput(config, logger, &KinesisOutputSettings{
		StreamName: settings.StreamName,
	})
}

type redisListOutputConfiguration struct {
	Project     string `cfg:"project"`
	Family      string `cfg:"family"`
	Application string `cfg:"application"`
	ServerName  string `cfg:"server_name" default:"default" validate:"required,min=1"`
	Key         string `cfg:"key" validate:"required,min=1"`
	BatchSize   int    `cfg:"batch_size" default:"100"`
}

func newRedisListOutputFromConfig(_ context.Context, config cfg.Config, logger log.Logger, name string) (Output, error) {
	key := ConfigurableOutputKey(name)

	configuration := redisListOutputConfiguration{}
	config.UnmarshalKey(key, &configuration)

	return NewRedisListOutput(config, logger, &RedisListOutputSettings{
		AppId: cfg.AppId{
			Project:     configuration.Project,
			Family:      configuration.Family,
			Application: configuration.Application,
		},
		ServerName: configuration.ServerName,
		Key:        configuration.Key,
		BatchSize:  configuration.BatchSize,
	})
}

type SnsOutputConfiguration struct {
	BaseOutputSettings
	Type        string `cfg:"type"`
	Project     string `cfg:"project"`
	Family      string `cfg:"family"`
	Application string `cfg:"application"`
	TopicId     string `cfg:"topic_id" validate:"required"`
	ClientName  string `cfg:"client_name"`
}

func newSnsOutputFromConfig(ctx context.Context, config cfg.Config, logger log.Logger, name string) (Output, error) {
	key := ConfigurableOutputKey(name)
	configuration := SnsOutputConfiguration{}
	config.UnmarshalKey(key, &configuration)

	clientName := configuration.ClientName
	if clientName == "" {
		clientName = fmt.Sprintf("stream-output-%s", name)
	}

	return NewSnsOutput(ctx, config, logger, &SnsOutputSettings{
		AppId: cfg.AppId{
			Project:     configuration.Project,
			Family:      configuration.Family,
			Application: configuration.Application,
		},
		TopicId:    configuration.TopicId,
		ClientName: clientName,
	})
}

type sqsOutputConfiguration struct {
	BaseOutputSettings
	Project           string            `cfg:"project"`
	Family            string            `cfg:"family"`
	Application       string            `cfg:"application"`
	QueueId           string            `cfg:"queue_id" validate:"required"`
	VisibilityTimeout int               `cfg:"visibility_timeout" default:"30" validate:"gt=0"`
	RedrivePolicy     sqs.RedrivePolicy `cfg:"redrive_policy"`
	Fifo              sqs.FifoSettings  `cfg:"fifo"`
	ClientName        string            `cfg:"client_name"`
}

func newSqsOutputFromConfig(ctx context.Context, config cfg.Config, logger log.Logger, name string) (Output, error) {
	key := ConfigurableOutputKey(name)
	configuration := sqsOutputConfiguration{}
	config.UnmarshalKey(key, &configuration)

	clientName := configuration.ClientName
	if clientName == "" {
		clientName = fmt.Sprintf("stream-output-%s", name)
	}

	return NewSqsOutput(ctx, config, logger, &SqsOutputSettings{
		AppId: cfg.AppId{
			Project:     configuration.Project,
			Family:      configuration.Family,
			Application: configuration.Application,
		},
		QueueId:           configuration.QueueId,
		VisibilityTimeout: configuration.VisibilityTimeout,
		RedrivePolicy:     configuration.RedrivePolicy,
		Fifo:              configuration.Fifo,
		ClientName:        clientName,
	})
}

func ConfigurableOutputKey(name string) string {
	return fmt.Sprintf("stream.output.%s", name)
}
