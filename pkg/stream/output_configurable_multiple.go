package stream

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/justtrackio/gosoline/pkg/cfg"
	"github.com/justtrackio/gosoline/pkg/log"
)

type multiOutput struct {
	outputs []Output
}

func (m *multiOutput) WriteOne(ctx context.Context, msg WritableMessage) error {
	err := &multierror.Error{}

	for _, output := range m.outputs {
		err = multierror.Append(err, output.WriteOne(ctx, msg))
	}

	return err.ErrorOrNil()
}

func (m *multiOutput) Write(ctx context.Context, batch []WritableMessage) error {
	err := &multierror.Error{}

	for _, output := range m.outputs {
		err = multierror.Append(err, output.Write(ctx, batch))
	}

	return err.ErrorOrNil()
}

func NewConfigurableMultiOutput(ctx context.Context, config cfg.Config, logger log.Logger, base string) (Output, error) {
	key := fmt.Sprintf("%s.types", ConfigurableOutputKey(base))
	ts := config.Get(key).(map[string]interface{})

	multiOutput := &multiOutput{
		outputs: make([]Output, 0),
	}

	for outputName := range ts {
		name := fmt.Sprintf("%s.types.%s", base, outputName)

		if output, err := NewConfigurableOutput(ctx, config, logger, name); err != nil {
			return nil, fmt.Errorf("can not create multi output %s: %w", base, err)
		} else {
			multiOutput.outputs = append(multiOutput.outputs, output)
		}
	}

	return multiOutput, nil
}
