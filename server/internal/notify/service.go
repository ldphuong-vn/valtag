package notify

import (
	"context"
	"fmt"
	"time"
)

// Notifier sends notifications.
type Notifier interface {
	Send(ctx context.Context, to, subject, body string) error
}

// Service routes notifications through available notifiers.
type Service struct {
	email    Notifier
	tokens   *ActionTokenStore
	baseURL  string
	slack    *SlackAdapter
	telegram *TelegramAdapter
	channels *ChannelStore
	jobStore *JobStore
}

// NewService creates a notification Service. Pass nil adapters to disable those channels.
func NewService(email Notifier, tokens *ActionTokenStore, baseURL string, slack *SlackAdapter, telegram *TelegramAdapter, channels *ChannelStore, jobStore *JobStore) *Service {
	return &Service{email: email, tokens: tokens, baseURL: baseURL, slack: slack, telegram: telegram, channels: channels, jobStore: jobStore}
}

// NotifyApprovalNeeded sends an approval email and/or Slack DM with one-click approve/reject.
func (s *Service) NotifyApprovalNeeded(ctx context.Context, ownerUserID, to, requestID, secretName, requester, reason string) error {
	approveToken, err := s.tokens.Create(ctx, requestID, "approve", 72*time.Hour)
	if err != nil {
		return fmt.Errorf("creating approve token: %w", err)
	}
	rejectToken, err := s.tokens.Create(ctx, requestID, "reject", 72*time.Hour)
	if err != nil {
		return fmt.Errorf("creating reject token: %w", err)
	}

	approveURL := s.baseURL + "/api/v1/action-tokens/" + approveToken + "/redeem?action=approve"
	rejectURL := s.baseURL + "/api/v1/action-tokens/" + rejectToken + "/redeem?action=reject"

	// Email notification
	if to != "" {
		subject := "Valt: Access Request Needs Your Approval"
		body := buildApprovalEmail(secretName, requester, reason, approveURL, rejectURL)
		if s.jobStore != nil {
			_ = s.jobStore.Enqueue(ctx, "email", to, subject, map[string]string{"body": body})
		} else if s.email != nil {
			if emailErr := s.email.Send(ctx, to, subject, body); emailErr != nil {
				fmt.Printf("notify: email send error: %v\n", emailErr)
			}
		}
	}

	// Slack notification
	if s.channels != nil && ownerUserID != "" {
		if slackUserID, chErr := s.channels.GetPreferred(ctx, ownerUserID, "slack"); chErr == nil && slackUserID != "" {
			payload := map[string]string{"request_id": requestID, "secret_name": secretName, "requester": requester, "reason": reason}
			if s.jobStore != nil {
				_ = s.jobStore.Enqueue(ctx, "slack", slackUserID, "", payload)
			} else if s.slack != nil {
				_ = s.slack.SendApprovalRequest(ctx, slackUserID, requestID, secretName, requester, reason)
			}
		}
	}

	// Telegram notification
	if s.channels != nil && ownerUserID != "" {
		if telegramHandle, chErr := s.channels.GetPreferred(ctx, ownerUserID, "telegram"); chErr == nil && telegramHandle != "" {
			var chatID int64
			_, _ = fmt.Sscan(telegramHandle, &chatID) // parse failure leaves chatID 0, skipped below
			if chatID != 0 {
				payload := map[string]interface{}{"chat_id": chatID, "request_id": requestID, "secret_name": secretName, "requester": requester, "reason": reason}
				if s.jobStore != nil {
					_ = s.jobStore.Enqueue(ctx, "telegram", telegramHandle, "", payload)
				} else if s.telegram != nil {
					_ = s.telegram.SendApprovalRequest(ctx, chatID, requestID, secretName, requester, reason)
				}
			}
		}
	}

	return nil
}

// NotifyAccessGranted sends an access-granted notification.
func (s *Service) NotifyAccessGranted(ctx context.Context, to, secretName string, durationMin int) error {
	if to == "" {
		return nil
	}
	subject := "Valt: Access Granted"
	body := "Access to secret '" + secretName + "' has been granted.\nDuration: " + intToStr(durationMin) + " minutes."
	if s.jobStore != nil {
		return s.jobStore.Enqueue(ctx, "email", to, subject, map[string]string{"body": body})
	}
	if s.email != nil {
		return s.email.Send(ctx, to, subject, body)
	}
	return nil
}

func intToStr(n int) string {
	if n <= 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
