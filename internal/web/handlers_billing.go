package web

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"writekit/internal/platform"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
)

func (h *Handler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	event, err := webhook.ConstructEvent(body, r.Header.Get("Stripe-Signature"), h.Config.StripeWebhookSecret)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			slog.Error("unmarshal checkout session", "err", err)
			return
		}
		h.handleCheckoutComplete(r, &sess)

	case "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			slog.Error("unmarshal subscription", "err", err)
			return
		}
		h.handleSubscriptionUpdate(r, &sub)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handleCheckoutComplete(r *http.Request, sess *stripe.CheckoutSession) {
	userID := sess.ClientReferenceID
	sub := &platform.Subscription{
		UserID:               userID,
		StripeCustomerID:     sess.Customer.ID,
		StripeSubscriptionID: sess.Subscription.ID,
		Status:               "active",
	}
	if err := h.DB.UpsertSubscription(r.Context(), sub); err != nil {
		slog.Error("upsert subscription", "err", err)
	}
}

func (h *Handler) handleSubscriptionUpdate(r *http.Request, sub *stripe.Subscription) {
	existing, err := h.DB.GetSubscriptionByCustomerID(r.Context(), sub.Customer.ID)
	if err != nil {
		slog.Error("get subscription by customer", "err", err)
		return
	}

	existing.Status = string(sub.Status)
	if err := h.DB.UpsertSubscription(r.Context(), existing); err != nil {
		slog.Error("update subscription", "err", err)
	}
}
