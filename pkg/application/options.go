package application

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/justtrackio/gosoline/pkg/apiserver"
	"github.com/justtrackio/gosoline/pkg/cfg"
	"github.com/justtrackio/gosoline/pkg/clock"
	"github.com/justtrackio/gosoline/pkg/db-repo"
	"github.com/justtrackio/gosoline/pkg/exec"
	"github.com/justtrackio/gosoline/pkg/fixtures"
	kernelPkg "github.com/justtrackio/gosoline/pkg/kernel"
	"github.com/justtrackio/gosoline/pkg/log"
	"github.com/justtrackio/gosoline/pkg/metric"
	"github.com/justtrackio/gosoline/pkg/stream"
	"github.com/justtrackio/gosoline/pkg/tracing"
	"github.com/pkg/errors"
)

type (
	Option       func(app *App)
	ConfigOption func(config cfg.GosoConf) error
	LoggerOption func(config cfg.GosoConf, logger log.GosoLogger) error
	KernelOption func(config cfg.GosoConf, kernel kernelPkg.GosoKernel) error
	SetupOption  func(config cfg.GosoConf, logger log.GosoLogger) error
)

type kernelSettings struct {
	KillTimeout time.Duration `cfg:"killTimeout" default:"10s"`
}

func WithApiHealthCheck(app *App) {
	WithModule("api-health-check", apiserver.NewApiHealthCheck())(app)
}

func WithConfigEnvKeyPrefix(prefix string) Option {
	return func(app *App) {
		app.addConfigOption(func(config cfg.GosoConf) error {
			prefix = strings.Replace(prefix, "-", "_", -1)

			return config.Option(cfg.WithEnvKeyPrefix(prefix))
		})
	}
}

func WithConfigEnvKeyReplacer(replacer *strings.Replacer) Option {
	return func(app *App) {
		app.addConfigOption(func(config cfg.GosoConf) error {
			if err := config.Option(cfg.WithEnvKeyReplacer(replacer)); err != nil {
				return err
			}

			return nil
		})
	}
}

func WithConfigErrorHandlers(handlers ...cfg.ErrorHandler) Option {
	return func(app *App) {
		app.addConfigOption(func(config cfg.GosoConf) error {
			return config.Option(cfg.WithErrorHandlers(handlers...))
		})
	}
}

func WithConfigFile(filePath string, fileType string) Option {
	return func(app *App) {
		app.addConfigOption(func(config cfg.GosoConf) error {
			return config.Option(cfg.WithConfigFile(filePath, fileType))
		})
	}
}

func WithConfigFileFlag(app *App) {
	app.addConfigOption(func(config cfg.GosoConf) error {
		flags := flag.NewFlagSet("cfg", flag.ContinueOnError)

		configFile := flags.String("config", "", "path to a config file")
		err := flags.Parse(os.Args[1:])
		if err != nil {
			return err
		}

		return config.Option(cfg.WithConfigFile(*configFile, "yml"))
	})
}

func WithConfigMap(configMap map[string]interface{}, mergeOptions ...cfg.MergeOption) Option {
	return func(app *App) {
		app.addConfigOption(func(config cfg.GosoConf) error {
			return config.Option(cfg.WithConfigMap(configMap, mergeOptions...))
		})
	}
}

func WithConfigPostProcessor(processor cfg.PostProcessor) Option {
	return func(app *App) {
		app.configPostProcessors = append(app.configPostProcessors, processor)
	}
}

func WithConfigSanitizers(sanitizers ...cfg.Sanitizer) Option {
	return func(app *App) {
		app.addConfigOption(func(config cfg.GosoConf) error {
			return config.Option(cfg.WithSanitizers(sanitizers...))
		})
	}
}

func WithConfigSetting(key string, settings interface{}) Option {
	return func(app *App) {
		app.addConfigOption(func(config cfg.GosoConf) error {
			return config.Option(cfg.WithConfigSetting(key, settings))
		})
	}
}

func WithConsumerMessagesPerRunnerMetrics(app *App) {
	app.addKernelOption(func(config cfg.GosoConf, kernel kernelPkg.GosoKernel) error {
		kernel.AddFactory(stream.MessagesPerRunnerMetricWriterFactory)
		return nil
	})
}

func WithExecBackoffInfinite(app *App) {
	app.addConfigOption(func(config cfg.GosoConf) error {
		return config.Option(cfg.WithConfigSetting("exec.backoff.type", "infinite"))
	})
}

func WithExecBackoffSettings(settings *exec.BackoffSettings) Option {
	return func(app *App) {
		app.addConfigOption(func(config cfg.GosoConf) error {
			return config.Option(cfg.WithConfigSetting("exec.backoff", settings, cfg.SkipExisting))
		})
	}
}

func WithDbRepoChangeHistory(app *App) {
	app.addKernelOption(func(config cfg.GosoConf, kernel kernelPkg.GosoKernel) error {
		kernel.AddMiddleware(db_repo.KernelMiddlewareChangeHistory, kernelPkg.PositionEnd)
		return nil
	})
}

func WithFixtures(fixtureSets []*fixtures.FixtureSet) Option {
	return func(app *App) {
		app.addKernelOption(func(config cfg.GosoConf, kernel kernelPkg.GosoKernel) error {
			kernel.AddMiddleware(fixtures.KernelMiddlewareLoader(fixtureSets), kernelPkg.PositionBeginning)
			return nil
		})
	}
}

func WithKernelSettingsFromConfig(app *App) {
	app.addKernelOption(func(config cfg.GosoConf, k kernelPkg.GosoKernel) error {
		settings := &kernelSettings{}
		config.UnmarshalKey("kernel", settings)

		return k.Option(kernelPkg.KillTimeout(settings.KillTimeout))
	})
}

func WithLoggerApplicationTag(app *App) {
	app.addLoggerOption(func(config cfg.GosoConf, logger log.GosoLogger) error {
		if !config.IsSet("app_name") {
			return errors.New("can not get application name from config to set it on logger")
		}

		return logger.Option(log.WithFields(map[string]interface{}{
			"application": config.GetString("app_name"),
		}))
	})
}

func WithLoggerContextFieldsMessageEncoder(app *App) {
	app.addLoggerOption(func(config cfg.GosoConf, logger log.GosoLogger) error {
		stream.AddDefaultEncodeHandler(log.NewMessageWithLoggingFieldsEncoder(config, logger))
		return nil
	})
}

func WithLoggerContextFieldsResolver(resolver ...log.ContextFieldsResolver) Option {
	return func(app *App) {
		app.addLoggerOption(func(config cfg.GosoConf, logger log.GosoLogger) error {
			return logger.Option(log.WithContextFieldsResolver(resolver...))
		})
	}
}

func WithLoggerHandlers(handler ...log.Handler) Option {
	return func(app *App) {
		app.addLoggerOption(func(config cfg.GosoConf, logger log.GosoLogger) error {
			return logger.Option(log.WithHandlers(handler...))
		})
	}
}

func WithLoggerHandlersFromConfig(app *App) {
	app.addLoggerOption(func(config cfg.GosoConf, logger log.GosoLogger) error {
		var err error
		var handlers []log.Handler

		if handlers, err = log.NewHandlersFromConfig(config); err != nil {
			return fmt.Errorf("can not create handlers from config: %w", err)
		}

		return logger.Option(log.WithHandlers(handlers...))
	})
}

func WithLoggerMetricHandler(app *App) {
	app.addLoggerOption(func(config cfg.GosoConf, logger log.GosoLogger) error {
		metricHandler := metric.NewLoggerHandler()
		return logger.Option(log.WithHandlers(metricHandler))
	})
}

func WithLoggerSentryHandler(contextProvider ...log.SentryContextProvider) Option {
	return func(app *App) {
		app.addLoggerOption(func(config cfg.GosoConf, logger log.GosoLogger) error {
			var err error
			var sentryHandler *log.HandlerSentry

			if sentryHandler, err = log.NewHandlerSentry(config); err != nil {
				return fmt.Errorf("can not create logger sentry handler: %w", err)
			}

			for _, provider := range contextProvider {
				if err = provider(config, sentryHandler); err != nil {
					return fmt.Errorf("can not run sentry context provider %T: %w", provider, err)
				}
			}

			return logger.Option(log.WithHandlers(sentryHandler))
		})
	}
}

func WithMetadataServer(app *App) {
	WithModule("metadata-server", NewMetadataServer())(app)
}

func WithMetricDaemon(app *App) {
	WithModule("metric", func(ctx context.Context, config cfg.Config, logger log.Logger) (kernelPkg.Module, error) {
		return metric.NewDaemon(ctx, config, logger)
	})(app)
}

func WithProducerDaemon(app *App) {
	app.addKernelOption(func(config cfg.GosoConf, kernel kernelPkg.GosoKernel) error {
		kernel.AddFactory(stream.ProducerDaemonFactory)
		return nil
	})
}

func WithProfiling() Option {
	return WithModule("profiling", apiserver.NewProfiling())
}

func WithTracing(app *App) {
	app.addLoggerOption(func(config cfg.GosoConf, logger log.GosoLogger) error {
		tracingHandler := tracing.NewLoggerErrorHandler()

		options := []log.Option{
			log.WithHandlers(tracingHandler),
			log.WithContextFieldsResolver(tracing.ContextTraceFieldsResolver),
		}

		return logger.Option(options...)
	})

	app.addSetupOption(func(config cfg.GosoConf, logger log.GosoLogger) error {
		strategy := tracing.NewTraceIdErrorWarningStrategy(logger)
		stream.AddDefaultEncodeHandler(tracing.NewMessageWithTraceEncoder(strategy))

		return nil
	})
}

func WithModule(name string, moduleFactory kernelPkg.ModuleFactory, opts ...kernelPkg.ModuleOption) Option {
	return func(app *App) {
		app.addKernelOption(func(config cfg.GosoConf, kernel kernelPkg.GosoKernel) error {
			kernel.Add(name, moduleFactory, opts...)

			return nil
		})
	}
}

func WithUTCClock(useUTC bool) Option {
	return func(app *App) {
		app.addSetupOption(func(config cfg.GosoConf, logger log.GosoLogger) error {
			clock.WithUseUTC(useUTC)

			return nil
		})
	}
}
