package mdlsub

import (
	"context"
	"fmt"

	"github.com/justtrackio/gosoline/pkg/apiserver"
	"github.com/justtrackio/gosoline/pkg/cfg"
	"github.com/justtrackio/gosoline/pkg/kernel"
	"github.com/justtrackio/gosoline/pkg/log"
	"github.com/justtrackio/gosoline/pkg/stream"
)

const (
	ConfigKeyMdlSub = "mdlsub"
)

type Settings struct {
	SubscriberApi SubscriberApiSettings          `cfg:"subscriber_api"`
	Subscribers   map[string]*SubscriberSettings `cfg:"subscribers"`
}

func NewSubscriberFactory(transformerFactoryMap TransformerMapTypeVersionFactories) kernel.MultiModuleFactory {
	return func(ctx context.Context, config cfg.Config, logger log.Logger) (map[string]kernel.ModuleFactory, error) {
		return SubscriberFactory(ctx, config, logger, transformerFactoryMap)
	}
}

func SubscriberFactory(ctx context.Context, config cfg.Config, logger log.Logger, transformerFactories TransformerMapTypeVersionFactories) (map[string]kernel.ModuleFactory, error) {
	settings := Settings{
		Subscribers: make(map[string]*SubscriberSettings),
	}
	config.UnmarshalKey(fmt.Sprintf("%s.%s", ConfigKeyMdlSub, "subscribers"), &settings.Subscribers)
	config.UnmarshalKey(fmt.Sprintf("%s.%s", ConfigKeyMdlSub, "subscriber_api"), &settings.SubscriberApi)

	var err error
	var transformers ModelTransformers
	var outputs Outputs

	if transformers, err = initTransformers(ctx, config, logger, settings.Subscribers, transformerFactories); err != nil {
		return nil, fmt.Errorf("can not create subscribers: %w", err)
	}

	if outputs, err = initOutputs(ctx, config, logger, settings.Subscribers, transformers); err != nil {
		return nil, fmt.Errorf("can not create subscribers: %w", err)
	}

	modules := make(map[string]kernel.ModuleFactory)

	for name := range settings.Subscribers {
		moduleName := fmt.Sprintf("subscriber-%s", name)
		consumerName := fmt.Sprintf("subscriber-%s", name)
		callbackFactory := NewSubscriberCallbackFactory(transformers, outputs)

		modules[moduleName] = stream.NewConsumer(consumerName, callbackFactory)
	}

	if !settings.SubscriberApi.Enabled {
		return modules, nil
	}

	callbackFactories := make(map[string]stream.ConsumerCallbackFactory)

	for name := range settings.Subscribers {
		settings := settings.Subscribers[name]

		model := settings.SourceModel.Name
		callbackFactory := NewSubscriberCallbackFactory(transformers, outputs)

		callbackFactories[model] = callbackFactory
	}

	definer := CreateDefiner(callbackFactories)
	modules["mdlsub_subscriberapi"] = apiserver.New(definer)

	return modules, nil
}
