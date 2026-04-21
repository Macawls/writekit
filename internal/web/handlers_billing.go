package web

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
	"writekit/internal/httplog"
	"writekit/internal/platform"
)

func (h *Handler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		log.Warn("stripe webhook: read body", "err", err)
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	event, err := webhook.ConstructEvent(body, r.Header.Get("Stripe-Signature"), h.Config.StripeWebhookSecret)
	if err != nil {
		log.Warn("stripe webhook: invalid signature", "err", err)
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	log.Info("stripe webhook received", "type", event.Type, "event_id", event.ID)

	switch event.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			log.Error("stripe webhook: unmarshal checkout session", "event_id", event.ID, "err", err)
			return
		}
		h.handleCheckoutComplete(r, &sess)

	case "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			log.Error("stripe webhook: unmarshal subscription", "event_id", event.ID, "err", err)
			return
		}
		h.handleSubscriptionUpdate(r, &sub)

	default:
		log.Debug("stripe webhook: unhandled event type", "type", event.Type)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handleCheckoutComplete(r *http.Request, sess *stripe.CheckoutSession) {
	log := httplog.FromContext(r.Context())
	userID := sess.ClientReferenceID
	sub := &platform.Subscription{
		UserID:               userID,
		StripeCustomerID:     sess.Customer.ID,
		StripeSubscriptionID: sess.Subscription.ID,
		Status:               "active",
	}
	if err := h.DB.UpsertSubscription(r.Context(), sub); err != nil {
		log.Error("stripe: upsert subscription", "user_id", userID, "customer_id", sess.Customer.ID, "err", err)
		return
	}
	log.Info("stripe: subscription activated", "user_id", userID, "customer_id", sess.Customer.ID)
}

func (h *Handler) handleSubscriptionUpdate(r *http.Request, sub *stripe.Subscription) {
	log := httplog.FromContext(r.Context())
	existing, err := h.DB.GetSubscriptionByCustomerID(r.Context(), sub.Customer.ID)
	if err != nil {
		log.Error("stripe: get subscription by customer", "customer_id", sub.Customer.ID, "err", err)
		return
	}

	existing.Status = string(sub.Status)
	if err := h.DB.UpsertSubscription(r.Context(), existing); err != nil {
		log.Error("stripe: update subscription", "customer_id", sub.Customer.ID, "status", sub.Status, "err", err)
		return
	}
	log.Info("stripe: subscription updated", "customer_id", sub.Customer.ID, "status", sub.Status)
}
