package stream_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/justtrackio/gosoline/pkg/cfg"
	"github.com/justtrackio/gosoline/pkg/encoding/json"
	logMocks "github.com/justtrackio/gosoline/pkg/log/mocks"
	"github.com/justtrackio/gosoline/pkg/mdl"
	metricMocks "github.com/justtrackio/gosoline/pkg/metric/mocks"
	"github.com/justtrackio/gosoline/pkg/stream"
	"github.com/justtrackio/gosoline/pkg/stream/mocks"
	"github.com/justtrackio/gosoline/pkg/tracing"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ConsumerTestSuite struct {
	suite.Suite

	data chan *stream.Message
	once sync.Once
	stop func()

	input *mocks.Input

	callback *mocks.RunnableConsumerCallback
	consumer *stream.Consumer
}

func (s *ConsumerTestSuite) SetupTest() {
	s.data = make(chan *stream.Message, 10)
	s.once = sync.Once{}
	s.stop = func() {
		s.once.Do(func() {
			close(s.data)
		})
	}

	s.input = new(mocks.Input)
	s.callback = new(mocks.RunnableConsumerCallback)

	logger := logMocks.NewLoggerMockedAll()
	tracer := tracing.NewNoopTracer()
	mw := metricMocks.NewWriterMockedAll()
	me := stream.NewMessageEncoder(&stream.MessageEncoderSettings{})
	settings := &stream.ConsumerSettings{
		Input:       "test",
		RunnerCount: 1,
		IdleTimeout: time.Second,
	}

	baseConsumer := stream.NewBaseConsumerWithInterfaces(logger, mw, tracer, s.input, me, s.callback, settings, "test", cfg.AppId{})
	s.consumer = stream.NewConsumerWithInterfaces(baseConsumer, s.callback)
}

func (s *ConsumerTestSuite) TestGetModelNil() {
	s.input.On("Data").Return(s.data)
	s.input.On("Run", mock.AnythingOfType("*context.cancelCtx")).Run(func(args mock.Arguments) {
		s.data <- stream.NewJsonMessage(`"foo"`, map[string]interface{}{
			"bla": "blub",
		})
		s.stop()
	}).Return(nil)
	s.input.On("Stop").Once()

	s.callback.On("GetModel", mock.AnythingOfType("map[string]interface {}")).Return(func(_ map[string]interface{}) interface{} {
		return nil
	})
	s.callback.On("Run", mock.AnythingOfType("*context.cancelCtx")).Return(nil)

	err := s.consumer.Run(context.Background())

	s.NoError(err, "there should be no error during run")
	s.input.AssertExpectations(s.T())
	s.callback.AssertExpectations(s.T())
}

func (s *ConsumerTestSuite) TestRun() {
	s.input.On("Data").Return(s.data)
	s.input.On("Run", mock.AnythingOfType("*context.cancelCtx")).Run(func(args mock.Arguments) {
		s.data <- stream.NewJsonMessage(`"foo"`)
		s.data <- stream.NewJsonMessage(`"bar"`)
		s.data <- stream.NewJsonMessage(`"foobar"`)
		s.stop()
	}).Return(nil)
	s.input.On("Stop").Once()

	consumed := make([]*string, 0)
	s.callback.On("Consume", mock.AnythingOfType("*context.cancelCtx"), mock.AnythingOfType("*string"), map[string]interface{}{}).
		Run(func(args mock.Arguments) {
			consumed = append(consumed, args[1].(*string))
		}).Return(true, nil)

	s.callback.On("GetModel", mock.AnythingOfType("map[string]interface {}")).
		Return(func(_ map[string]interface{}) interface{} {
			return mdl.String("")
		})

	s.callback.On("Run", mock.AnythingOfType("*context.cancelCtx")).Return(nil)

	err := s.consumer.Run(context.Background())

	s.NoError(err, "there should be no error during run")
	s.Len(consumed, 3)

	s.input.AssertExpectations(s.T())
	s.callback.AssertExpectations(s.T())
}

func (s *ConsumerTestSuite) TestRun_ContextCancel() {
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	once := sync.Once{}

	s.input.On("Data").
		Return(s.data)

	s.input.On("Run", mock.AnythingOfType("*context.cancelCtx")).
		Run(func(args mock.Arguments) {
			cancel()
			<-stopped
			s.stop()
		}).Return(nil)

	s.input.On("Stop").
		Run(func(args mock.Arguments) {
			once.Do(func() {
				close(stopped)
			})
		}).Once()

	s.callback.On("Run", mock.AnythingOfType("*context.cancelCtx")).
		Return(nil)

	err := s.consumer.Run(ctx)

	s.NoError(err, "there should be no error during run")

	s.input.AssertExpectations(s.T())
	s.callback.AssertExpectations(s.T())
}

func (s *ConsumerTestSuite) TestRun_InputRunError() {
	s.input.
		On("Data").
		Return(s.data)

	s.input.
		On("Run", mock.AnythingOfType("*context.cancelCtx")).
		Return(fmt.Errorf("read error"))

	s.callback.
		On("Run", mock.AnythingOfType("*context.cancelCtx")).
		Run(func(args mock.Arguments) {
			<-args[0].(context.Context).Done()
		}).Return(nil)

	err := s.consumer.Run(context.Background())

	s.EqualError(err, "error while waiting for all routines to stop: panic during run of the consumer input: read error")

	s.input.AssertExpectations(s.T())
	s.callback.AssertExpectations(s.T())
}

func (s *ConsumerTestSuite) TestRun_CallbackRunError() {
	s.input.On("Data").
		Return(s.data)
	s.input.On("Stop").
		Once()

	s.input.On("Run", mock.AnythingOfType("*context.cancelCtx")).
		Run(func(args mock.Arguments) {
			<-args[0].(context.Context).Done()
		}).
		Return(nil)

	s.callback.On("Run", mock.AnythingOfType("*context.cancelCtx")).
		Return(fmt.Errorf("consumerCallback run error"))

	err := s.consumer.Run(context.Background())

	s.EqualError(err, "error while waiting for all routines to stop: panic during run of the consumerCallback: consumerCallback run error")

	s.input.AssertExpectations(s.T())
	s.callback.AssertExpectations(s.T())
}

func (s *ConsumerTestSuite) TestRun_CallbackRunPanic() {
	s.input.On("Data").
		Return(s.data)

	s.input.On("Run", mock.AnythingOfType("*context.cancelCtx")).
		Run(func(args mock.Arguments) {
			s.data <- stream.NewJsonMessage(`"foo"`)
			s.data <- stream.NewJsonMessage(`"bar"`)
			s.stop()
		}).Return(nil)

	s.input.
		On("Stop").
		Once()

	consumed := make([]*string, 0)

	s.callback.On("Run", mock.AnythingOfType("*context.cancelCtx")).
		Return(nil)

	s.callback.On("Consume", mock.AnythingOfType("*context.cancelCtx"), mock.AnythingOfType("*string"), map[string]interface{}{}).
		Run(func(args mock.Arguments) {
			ptr := args.Get(1).(*string)
			consumed = append(consumed, ptr)

			msg := *ptr
			if msg == "foo" {
				panic("foo")
			}
		}).
		Return(true, nil)

	s.callback.On("GetModel", mock.AnythingOfType("map[string]interface {}")).
		Return(func(_ map[string]interface{}) interface{} {
			return mdl.String("")
		})

	err := s.consumer.Run(context.Background())

	s.Nil(err, "there should be no error returned on consume")
	s.Len(consumed, 2)

	s.input.AssertExpectations(s.T())
	s.callback.AssertExpectations(s.T())
}

func (s *ConsumerTestSuite) TestRun_AggregateMessage() {
	message1 := stream.NewJsonMessage(`"foo"`, map[string]interface{}{
		"attr1": "a",
	})
	message2 := stream.NewJsonMessage(`"bar"`, map[string]interface{}{
		"attr1": "b",
	})

	aggregateBody, err := json.Marshal([]stream.WritableMessage{message1, message2})
	s.Require().NoError(err)

	aggregate := stream.BuildAggregateMessage(string(aggregateBody))

	s.input.On("Data").
		Return(s.data)

	s.input.On("Run", mock.AnythingOfType("*context.cancelCtx")).
		Run(func(args mock.Arguments) {
			s.data <- aggregate
			s.stop()
		}).Return(nil)

	s.input.On("Stop").
		Once()

	consumed := make([]string, 0)
	s.callback.On("Run", mock.AnythingOfType("*context.cancelCtx")).
		Return(nil)

	expectedAttributes1 := map[string]interface{}{"attr1": "a"}
	s.callback.On("Consume", mock.AnythingOfType("*context.cancelCtx"), mock.AnythingOfType("*string"), expectedAttributes1).
		Run(func(args mock.Arguments) {
			ptr := args.Get(1).(*string)
			consumed = append(consumed, *ptr)
		}).
		Return(true, nil)

	expectedModelAttributes1 := map[string]interface{}{"attr1": "a", "encoding": "application/json"}
	s.callback.On("GetModel", expectedModelAttributes1).
		Return(mdl.String(""))

	expectedAttributes2 := map[string]interface{}{"attr1": "b"}
	s.callback.On("Consume", mock.AnythingOfType("*context.cancelCtx"), mock.AnythingOfType("*string"), expectedAttributes2).
		Run(func(args mock.Arguments) {
			ptr := args.Get(1).(*string)
			consumed = append(consumed, *ptr)
		}).
		Return(true, nil)

	expectedModelAttributes2 := map[string]interface{}{"attr1": "b", "encoding": "application/json"}
	s.callback.On("GetModel", expectedModelAttributes2).
		Return(mdl.String(""))

	err = s.consumer.Run(context.Background())

	s.Nil(err, "there should be no error returned on consume")
	s.Len(consumed, 2)
	s.Equal("foobar", strings.Join(consumed, ""))

	s.input.AssertExpectations(s.T())
	s.callback.AssertExpectations(s.T())
}

func TestConsumerTestSuite(t *testing.T) {
	suite.Run(t, new(ConsumerTestSuite))
}
