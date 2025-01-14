package stream

import (
	"context"

	"github.com/justtrackio/gosoline/pkg/log"
)

type ConsumerAcknowledge struct {
	logger log.Logger
	input  Input
}

func NewConsumerAcknowledgeWithInterfaces(logger log.Logger, input Input) ConsumerAcknowledge {
	return ConsumerAcknowledge{
		logger: logger,
		input:  input,
	}
}

func (c *ConsumerAcknowledge) Acknowledge(ctx context.Context, msg *Message) {
	var ok bool
	var ackInput AcknowledgeableInput

	if ackInput, ok = c.input.(AcknowledgeableInput); !ok {
		return
	}

	if err := ackInput.Ack(ctx, msg); err != nil {
		c.logger.WithContext(ctx).Error("could not acknowledge the message: %w", err)
	}
}

func (c *ConsumerAcknowledge) AcknowledgeBatch(ctx context.Context, msg []*Message) {
	var ok bool
	var ackInput AcknowledgeableInput

	if ackInput, ok = c.input.(AcknowledgeableInput); !ok {
		return
	}

	if err := ackInput.AckBatch(ctx, msg); err != nil {
		c.logger.WithContext(ctx).Error("could not acknowledge the messages: %w", err)
	}
}
