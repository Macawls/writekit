package web

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"writekit/internal/auth"
	"writekit/internal/platform"
	"github.com/stripe/stripe-go/v82"
	billingportal "github.com/stripe/stripe-go/v82/billingportal/session"
	checkout "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/webhook"
)

func (h *Handler) BillingPage(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	sub, _ := h.DB.GetSubscription(r.Context(), user.ID)

	h.Engine.Render(w, "billing.html", map[string]any{
		"User":         user,
		"Subscription": sub,
	})
}

func (h *Handler) BillingCheckout(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(h.Config.StripePriceID), Quantity: stripe.Int64(1)},
		},
		SuccessURL:        stripe.String(h.Config.BaseURL + "/billing?success=1"),
		CancelURL:         stripe.String(h.Config.BaseURL + "/billing?cancel=1"),
		CustomerEmail:     stripe.String(user.Email),
		ClientReferenceID: stripe.String(user.ID),
	}

	sess, err := checkout.New(params)
	if err != nil {
		slog.Error("create checkout session", "err", err)
		http.Error(w, "failed to create checkout", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, sess.URL, http.StatusSeeOther)
}

func (h *Handler) BillingPortal(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	sub, err := h.DB.GetSubscription(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "no subscription found", http.StatusBadRequest)
		return
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(sub.StripeCustomerID),
		ReturnURL: stripe.String(h.Config.BaseURL + "/billing"),
	}

	sess, err := billingportal.New(params)
	if err != nil {
		slog.Error("create portal session", "err", err)
		http.Error(w, "failed to create portal", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, sess.URL, http.StatusSeeOther)
}

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
		json.Unmarshal(event.Data.Raw, &sess)
		h.handleCheckoutComplete(r, &sess)

	case "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		json.Unmarshal(event.Data.Raw, &sub)
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
