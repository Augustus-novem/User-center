package twiliosms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	twiliogo "github.com/twilio/twilio-go"
	api "github.com/twilio/twilio-go/rest/api/v2010"
)

type Service struct {
	client              *twiliogo.RestClient
	messagingServiceSID string
	statusCallbackURL   string
}

func NewService(client *twiliogo.RestClient, messagingServiceSID string) *Service {
	return &Service{
		client:              client,
		messagingServiceSID: messagingServiceSID,
	}
}

func NewServiceFromEnv() (*Service, error) {
	accountSID := os.Getenv("TWILIO_ACCOUNT_SID")
	authToken := os.Getenv("TWILIO_AUTH_TOKEN")
	messagingServiceSID := os.Getenv("TWILIO_MESSAGING_SERVICE_SID")

	if accountSID == "" {
		return nil, errors.New("missing TWILIO_ACCOUNT_SID")
	}
	if authToken == "" {
		return nil, errors.New("missing TWILIO_AUTH_TOKEN")
	}
	if messagingServiceSID == "" {
		return nil, errors.New("missing TWILIO_MESSAGING_SERVICE_SID")
	}

	client := twiliogo.NewRestClientWithParams(twiliogo.ClientParams{
		Username: accountSID,
		Password: authToken,
	})

	return &Service{
		client:              client,
		messagingServiceSID: messagingServiceSID,
		statusCallbackURL:   os.Getenv("TWILIO_STATUS_CALLBACK_URL"), // 可为空
	}, nil
}

func (s *Service) Send(ctx context.Context, tplId string, args []string, numbers ...string) error {
	_ = ctx // 先保留你的接口形状；下面直接走 Twilio SDK

	if tplId == "" {
		return errors.New("twilio tplId is empty")
	}
	if len(numbers) == 0 {
		return errors.New("twilio numbers is empty")
	}
	if s.messagingServiceSID == "" {
		return errors.New("twilio messagingServiceSID is empty")
	}

	varsJSON, err := buildContentVariables(args)
	if err != nil {
		return err
	}

	for _, number := range numbers {
		params := &api.CreateMessageParams{}
		params.SetTo(number)
		params.SetMessagingServiceSid(s.messagingServiceSID)
		params.SetContentSid(tplId)

		if varsJSON != "" {
			params.SetContentVariables(varsJSON)
		}
		if s.statusCallbackURL != "" {
			params.SetStatusCallback(s.statusCallbackURL)
		}

		resp, err := s.client.Api.CreateMessage(params)
		if err != nil {
			return fmt.Errorf("twilio send to %s failed: %w", number, err)
		}
		if resp == nil || resp.Sid == nil || *resp.Sid == "" {
			return fmt.Errorf("twilio send to %s failed: empty message sid", number)
		}
	}

	return nil
}

func buildContentVariables(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}

	// Twilio ContentVariables 形如:
	// {"1":"123456","2":"5分钟"}
	m := make(map[string]string, len(args))
	for i, arg := range args {
		m[strconv.Itoa(i+1)] = arg
	}

	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("marshal content variables: %w", err)
	}
	return string(b), nil
}
