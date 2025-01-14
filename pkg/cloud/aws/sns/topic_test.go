package sns_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsSns "github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
	gosoSns "github.com/justtrackio/gosoline/pkg/cloud/aws/sns"
	gosoSnsMocks "github.com/justtrackio/gosoline/pkg/cloud/aws/sns/mocks"
	logMocks "github.com/justtrackio/gosoline/pkg/log/mocks"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/suite"
)

type TopicTestSuite struct {
	suite.Suite
	ctx    context.Context
	client *gosoSnsMocks.Client
	topic  gosoSns.Topic
}

func (s *TopicTestSuite) SetupTest() {
	logger := logMocks.NewLoggerMockedAll()

	s.ctx = context.Background()
	s.client = new(gosoSnsMocks.Client)
	s.topic = gosoSns.NewTopicWithInterfaces(logger, s.client, "topicArn")
}

func (s *TopicTestSuite) TestPublish() {
	input := &awsSns.PublishInput{
		TopicArn:          aws.String("topicArn"),
		Message:           aws.String("test"),
		MessageAttributes: map[string]types.MessageAttributeValue{},
	}

	s.client.On("Publish", s.ctx, input).Return(nil, nil)

	err := s.topic.Publish(s.ctx, "test", map[string]interface{}{})
	s.NoError(err)

	s.client.AssertExpectations(s.T())
}

func (s *TopicTestSuite) TestPublishError() {
	input := &awsSns.PublishInput{
		TopicArn: aws.String("topicArn"),
		Message:  aws.String("test"),
	}

	s.client.On("Publish", s.ctx, input).Return(nil, fmt.Errorf("error"))

	err := s.topic.Publish(context.Background(), "test")
	s.Error(err)

	s.client.AssertExpectations(s.T())
}

func (s *TopicTestSuite) TestSubscribeSqs() {
	listInput := &awsSns.ListSubscriptionsByTopicInput{TopicArn: aws.String("topicArn")}
	listOutput := &awsSns.ListSubscriptionsByTopicOutput{}
	s.client.On("ListSubscriptionsByTopic", s.ctx, listInput).Return(listOutput, nil)

	subInput := &awsSns.SubscribeInput{
		Attributes: map[string]string{
			"FilterPolicy": `{"model":"goso","version":1}`,
		},
		TopicArn: aws.String("topicArn"),
		Protocol: aws.String("sqs"),
		Endpoint: aws.String("queueArn"),
	}
	s.client.On("Subscribe", s.ctx, subInput).Return(nil, nil)

	err := s.topic.SubscribeSqs(s.ctx, "queueArn", map[string]interface{}{
		"model":   "goso",
		"version": 1,
	})
	s.NoError(err)

	s.client.AssertExpectations(s.T())
}

func (s *TopicTestSuite) TestSubscribeSqsExists() {
	listInput := &awsSns.ListSubscriptionsByTopicInput{TopicArn: aws.String("topicArn")}
	listOutput := &awsSns.ListSubscriptionsByTopicOutput{
		Subscriptions: []types.Subscription{
			{
				TopicArn:        aws.String("topicArn"),
				SubscriptionArn: aws.String("subscriptionArn"),
				Endpoint:        aws.String("queueArn"),
			},
		},
	}
	s.client.On("ListSubscriptionsByTopic", s.ctx, listInput).Return(listOutput, nil)

	getAttributesInput := &awsSns.GetSubscriptionAttributesInput{SubscriptionArn: aws.String("subscriptionArn")}
	getAttributesOutput := &awsSns.GetSubscriptionAttributesOutput{
		Attributes: map[string]string{
			"FilterPolicy": `{"model":"goso","version":1}`,
		},
	}
	s.client.On("GetSubscriptionAttributes", s.ctx, getAttributesInput).Return(getAttributesOutput, nil)

	err := s.topic.SubscribeSqs(context.Background(), "queueArn", map[string]interface{}{
		"model":   "goso",
		"version": 1,
	})
	s.NoError(err)

	s.client.AssertExpectations(s.T())
}

func (s *TopicTestSuite) TestSubscribeSqsExistsWithDifferentAttributes() {
	listInput := &awsSns.ListSubscriptionsByTopicInput{TopicArn: aws.String("topicArn")}
	listOutput := &awsSns.ListSubscriptionsByTopicOutput{
		Subscriptions: []types.Subscription{
			{
				TopicArn:        aws.String("topicArn"),
				SubscriptionArn: aws.String("subscriptionArn"),
				Endpoint:        aws.String("queueArn"),
			},
		},
	}
	s.client.On("ListSubscriptionsByTopic", s.ctx, listInput).Return(listOutput, nil)

	getAttributesInput := &awsSns.GetSubscriptionAttributesInput{SubscriptionArn: aws.String("subscriptionArn")}
	getAttributesOutput := &awsSns.GetSubscriptionAttributesOutput{
		Attributes: map[string]string{
			"FilterPolicy": `{"model":"mismatch"}`,
		},
	}
	s.client.On("GetSubscriptionAttributes", s.ctx, getAttributesInput).Return(getAttributesOutput, nil)

	unsubscribeInput := &awsSns.UnsubscribeInput{SubscriptionArn: aws.String("subscriptionArn")}
	unsubscribeOutput := &awsSns.UnsubscribeOutput{}
	s.client.On("Unsubscribe", s.ctx, unsubscribeInput).Return(unsubscribeOutput, nil)

	subInput := &awsSns.SubscribeInput{
		Attributes: map[string]string{
			"FilterPolicy": `{"model":"goso"}`,
		},
		Endpoint: aws.String("queueArn"),
		Protocol: aws.String("sqs"),
		TopicArn: aws.String("topicArn"),
	}
	s.client.On("Subscribe", s.ctx, subInput).Return(nil, nil)

	err := s.topic.SubscribeSqs(context.Background(), "queueArn", map[string]interface{}{
		"model": "goso",
	})
	s.NoError(err)

	s.client.AssertExpectations(s.T())
}

func (s *TopicTestSuite) TestSubscribeSqsError() {
	subErr := errors.New("subscribe error")

	listInput := &awsSns.ListSubscriptionsByTopicInput{TopicArn: aws.String("topicArn")}
	listOutput := &awsSns.ListSubscriptionsByTopicOutput{}
	s.client.On("ListSubscriptionsByTopic", s.ctx, listInput).Return(listOutput, nil)

	subInput := &awsSns.SubscribeInput{
		Attributes: map[string]string{},
		TopicArn:   aws.String("topicArn"),
		Protocol:   aws.String("sqs"),
		Endpoint:   aws.String("queueArn"),
	}
	s.client.On("Subscribe", s.ctx, subInput).Return(nil, subErr)

	err := s.topic.SubscribeSqs(s.ctx, "queueArn", map[string]interface{}{})
	s.EqualError(err, "could not subscribe to topic arn topicArn for sqs queue arn queueArn: subscribe error")

	s.client.AssertExpectations(s.T())
}

func TestTopicTestSuite(t *testing.T) {
	suite.Run(t, new(TopicTestSuite))
}
