package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/jsvm"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/billingportal/session"
	checkoutSession "github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/webhook"
)

func coalesce(value *string, defaultValue string) string {
	if value != nil {
		return *value
	}
	return defaultValue
}

func int64ToISODate(timestamp int64) string {
	// Convert the Unix timestamp to a time.Time
	t := time.Unix(timestamp, 0)

	// Format the time as an ISO 8601 date string (in UTC)
	return t.Format(time.RFC3339)
}

func main() {
	app := pocketbase.New()

	// Retrieve your STRIPE_SECRET_KEY from environment variables
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	stripeSuccessURL := os.Getenv("STRIPE_SUCCESS_URL")
	stripeCancelURL := os.Getenv("STRIPE_CANCEL_URL")
	stripeBillingReturnURL := os.Getenv("STRIPE_BILLING_RETURN_URL")
	WHSEC := os.Getenv("STRIPE_WHSEC")

	// Register JSVM plugin for JavaScript hooks
	jsvm.MustRegister(app, jsvm.Config{
		HooksWatch:    true,
		HooksPoolSize: 25,
	})

	// Register all routes in a single OnServe hook
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// Simple test endpoint
		se.Router.GET("/goext/{name}", func(e *core.RequestEvent) error {
			name := e.Request.PathValue("name")
			return e.JSON(http.StatusOK, map[string]string{"message": "Hello " + name})
		})

		// Create checkout session endpoint
		se.Router.POST("/create-checkout-session", func(e *core.RequestEvent) error {
			// 1. Destructure the price and quantity from the POST body
			payload, err := io.ReadAll(e.Request.Body)
			if err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"failure": "Could not read request body"})
			}
			var data map[string]interface{}
			if err := json.Unmarshal(payload, &data); err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"failure": "Could not parse request body"})
			}

			price, _ := data["price"].(map[string]interface{})
			quantity, _ := data["quantity"].(float64)

			// 2. Get the user from pocketbase auth
			token := e.Request.Header.Get("Authorization")
			record, err := app.FindAuthRecordByToken(token, core.TokenTypeAuth)
			if err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"failure": "Could not get user"})
			}

			// 3. Retrieve or create the customer in Stripe
			existingCustomerRecord, err := app.FindFirstRecordByData("customer", "user_id", record.Id)
			if err != nil {
				// Create new customer if none exists
				customerEmail := record.GetString("email")
				customerParams := &stripe.CustomerParams{
					Email: &customerEmail,
					Metadata: map[string]string{
						"pocketbaseUUID": record.GetString("id"),
					},
				}

				stripeCustomer, err := customer.New(customerParams)
				if err != nil {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "Could not create Stripe customer"})
				}

				// Upload customer to pocketbase
				collection, err := app.FindCollectionByNameOrId("customer")
				if err != nil {
					return err
				}

				newCustomerRecord := core.NewRecord(collection)
				newCustomerRecord.Set("user_id", record.Id)
				newCustomerRecord.Set("stripe_customer_id", stripeCustomer.ID)

				if err := app.Save(newCustomerRecord); err != nil {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "Could not create new customer"})
				}

				// Do Pricing New Customer
				if price["type"] == "recurring" {
					lineParams := []*stripe.CheckoutSessionLineItemParams{
						{
							Price:    stripe.String(price["id"].(string)),
							Quantity: stripe.Int64(int64(quantity)),
						},
					}
					customerUpdateParams := &stripe.CheckoutSessionCustomerUpdateParams{
						Address: stripe.String("auto"),
					}
					subscriptionParams := &stripe.CheckoutSessionSubscriptionDataParams{
						Metadata: map[string]string{},
					}

					sessionParams := &stripe.CheckoutSessionParams{
						Customer:                 &stripeCustomer.ID,
						PaymentMethodTypes:       stripe.StringSlice([]string{"card"}),
						BillingAddressCollection: stripe.String("required"),
						CustomerUpdate:           customerUpdateParams,
						Mode:                     stripe.String("subscription"),
						AllowPromotionCodes:      stripe.Bool(true),
						SuccessURL:               &stripeSuccessURL,
						CancelURL:                &stripeCancelURL,
						LineItems:                lineParams,
						SubscriptionData:         subscriptionParams,
					}
					sesh, _ := checkoutSession.New(sessionParams)
					return e.JSON(http.StatusOK, sesh)
				} else if price["type"] == "one_time" {
					lineParams := []*stripe.CheckoutSessionLineItemParams{
						{
							Price:    stripe.String(price["id"].(string)),
							Quantity: stripe.Int64(int64(quantity)),
						},
					}
					customerUpdateParams := &stripe.CheckoutSessionCustomerUpdateParams{
						Address: stripe.String("auto"),
					}

					sessionParams := &stripe.CheckoutSessionParams{
						Customer:                 &stripeCustomer.ID,
						PaymentMethodTypes:       stripe.StringSlice([]string{"card"}),
						BillingAddressCollection: stripe.String("required"),
						CustomerUpdate:           customerUpdateParams,
						Mode:                     stripe.String("payment"),
						AllowPromotionCodes:      stripe.Bool(true),
						SuccessURL:               &stripeSuccessURL,
						CancelURL:                &stripeCancelURL,
						LineItems:                lineParams,
					}
					sesh, _ := checkoutSession.New(sessionParams)
					return e.JSON(http.StatusOK, sesh)
				} else {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "Could not create new session"})
				}
			} else {
				// Do Pricing Existing Customer
				if price["type"] == "recurring" {
					lineParams := []*stripe.CheckoutSessionLineItemParams{
						{
							Price:    stripe.String(price["id"].(string)),
							Quantity: stripe.Int64(int64(quantity)),
						},
					}
					customerUpdateParams := &stripe.CheckoutSessionCustomerUpdateParams{
						Address: stripe.String("auto"),
					}
					subscriptionParams := &stripe.CheckoutSessionSubscriptionDataParams{
						Metadata: map[string]string{},
					}

					sessionParams := &stripe.CheckoutSessionParams{
						Customer:                 stripe.String(existingCustomerRecord.GetString("stripe_customer_id")),
						PaymentMethodTypes:       stripe.StringSlice([]string{"card"}),
						BillingAddressCollection: stripe.String("required"),
						CustomerUpdate:           customerUpdateParams,
						Mode:                     stripe.String("subscription"),
						AllowPromotionCodes:      stripe.Bool(true),
						SuccessURL:               &stripeSuccessURL,
						CancelURL:                &stripeCancelURL,
						LineItems:                lineParams,
						SubscriptionData:         subscriptionParams,
					}
					sesh, _ := checkoutSession.New(sessionParams)
					return e.JSON(http.StatusOK, sesh)
				} else if price["type"] == "one_time" {
					lineParams := []*stripe.CheckoutSessionLineItemParams{
						{
							Price:    stripe.String(price["id"].(string)),
							Quantity: stripe.Int64(int64(quantity)),
						},
					}
					customerUpdateParams := &stripe.CheckoutSessionCustomerUpdateParams{
						Address: stripe.String("auto"),
					}

					sessionParams := &stripe.CheckoutSessionParams{
						Customer:                 stripe.String(existingCustomerRecord.GetString("stripe_customer_id")),
						PaymentMethodTypes:       stripe.StringSlice([]string{"card"}),
						BillingAddressCollection: stripe.String("required"),
						CustomerUpdate:           customerUpdateParams,
						Mode:                     stripe.String("payment"),
						AllowPromotionCodes:      stripe.Bool(true),
						SuccessURL:               &stripeSuccessURL,
						CancelURL:                &stripeCancelURL,
						LineItems:                lineParams,
					}
					sesh, _ := checkoutSession.New(sessionParams)
					return e.JSON(http.StatusOK, sesh)
				} else {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "Could not create new session for stripe"})
				}
			}
		})

		// Create portal link endpoint
		se.Router.POST("/create-portal-link", func(e *core.RequestEvent) error {
			// 1. Get the user from pocketbase auth
			token := e.Request.Header.Get("Authorization")
			record, err := app.FindAuthRecordByToken(token, core.TokenTypeAuth)
			if err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"failure": "Could not get user"})
			}

			// 2. Retrieve or create the customer in Stripe
			existingCustomerRecord, err := app.FindFirstRecordByData("customer", "user_id", record.Id)
			if err != nil {
				// Create new customer if none exists
				customerParams := &stripe.CustomerParams{
					Metadata: map[string]string{
						"pocketbaseUUID": record.GetString("id"),
					},
				}

				stripeCustomer, err := customer.New(customerParams)
				if err != nil {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "Could not create Stripe customer"})
				}

				// Upload customer to pocketbase
				collection, err := app.FindCollectionByNameOrId("customer")
				if err != nil {
					return err
				}

				newCustomerRecord := core.NewRecord(collection)
				newCustomerRecord.Set("user_id", record.Id)
				newCustomerRecord.Set("stripe_customer_id", stripeCustomer.ID)

				if err := app.Save(newCustomerRecord); err != nil {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "Could not create new customer"})
				}

				// Create new session
				sessionParams := &stripe.BillingPortalSessionParams{
					Customer:  stripe.String(stripeCustomer.ID),
					ReturnURL: &stripeBillingReturnURL,
				}
				sesh, err := session.New(sessionParams)
				if err != nil {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "Could not create new session"})
				}
				return e.JSON(http.StatusOK, sesh)
			} else {
				// Create new session for existing customer
				sessionParams := &stripe.BillingPortalSessionParams{
					Customer:  stripe.String(existingCustomerRecord.GetString("stripe_customer_id")),
					ReturnURL: &stripeBillingReturnURL,
				}
				sesh, err := session.New(sessionParams)
				if err != nil {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "Could not create new session"})
				}
				return e.JSON(http.StatusOK, sesh)
			}
		})

		// Stripe webhook endpoint
		se.Router.POST("/stripe", func(e *core.RequestEvent) error {
			// Read the request body into a byte slice
			payload, err := io.ReadAll(e.Request.Body)
			if err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"failure": "failed to read body"})
			}

			event := stripe.Event{}
			err = json.Unmarshal(payload, &event)
			if err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"failure": "failed to parse JSON"})
			}

			signatureHeader := e.Request.Header.Get("Stripe-Signature")
			event, err = webhook.ConstructEvent(payload, signatureHeader, WHSEC)
			if err != nil {
				failureMessage := fmt.Sprintf("webhook verification failed: payload=%q, signatureHeader=%q, err=%q",
					payload, signatureHeader, err)
				return e.JSON(http.StatusBadRequest, map[string]string{"failure": failureMessage})
			}

			switch event.Type {
			case "product.created", "product.updated":
				var product stripe.Product
				err := json.Unmarshal(event.Data.Raw, &product)
				if err != nil {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "failed to marshall the stripe event"})
				}

				collection, err := app.FindCollectionByNameOrId("product")
				if err != nil {
					return err
				}

				existingRecord, err := app.FindFirstRecordByData("product", "product_id", product.ID)
				var recordToSave *core.Record

				if err == nil && existingRecord != nil {
					// Existing record found, update it
					recordToSave = existingRecord
				} else {
					// Existing record not found, insert a new record
					recordToSave = core.NewRecord(collection)
				}

				recordToSave.Set("product_id", product.ID)
				recordToSave.Set("active", product.Active)
				recordToSave.Set("name", product.Name)
				recordToSave.Set("description", coalesce(&product.Description, ""))
				recordToSave.Set("metadata", product.Metadata)

				if err := app.Save(recordToSave); err != nil {
					return err
				}

			case "price.created", "price.updated":
				var price stripe.Price
				err := json.Unmarshal(event.Data.Raw, &price)
				if err != nil {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "failed to marshall the stripe event"})
				}

				collection, err := app.FindCollectionByNameOrId("price")
				if err != nil {
					return err
				}

				existingRecord, err := app.FindFirstRecordByData("price", "price_id", price.ID)
				var recordToSave *core.Record

				if err == nil && existingRecord != nil {
					// Existing record found, update it
					recordToSave = existingRecord
				} else {
					// Existing record not found, insert a new record
					recordToSave = core.NewRecord(collection)
				}

				recordToSave.Set("price_id", price.ID)
				recordToSave.Set("product_id", price.Product.ID)
				recordToSave.Set("active", price.Active)
				recordToSave.Set("currency", price.Currency)
				recordToSave.Set("description", price.Nickname)
				recordToSave.Set("type", price.Type)
				recordToSave.Set("unit_amount", price.UnitAmount)
				recordToSave.Set("metadata", price.Metadata)

				// Check if Recurring is not nil before accessing its fields
				if price.Recurring != nil {
					recordToSave.Set("interval", price.Recurring.Interval)
					recordToSave.Set("interval_count", price.Recurring.IntervalCount)
					recordToSave.Set("trial_period_days", price.Recurring.TrialPeriodDays)
				}

				if err := app.Save(recordToSave); err != nil {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "failed to submit to pocketbase"})
				}

			case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
				var subscription stripe.Subscription
				err := json.Unmarshal(event.Data.Raw, &subscription)
				if err != nil {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "failed to marshall the stripe event"})
				}

				// Get customer's UUID from mapping table
				existingCustomer, err := app.FindFirstRecordByData("customer", "stripe_customer_id", subscription.Customer.ID)
				if err != nil {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "no customer"})
				}

				uuid := existingCustomer.GetString("user_id")
				collection, err := app.FindCollectionByNameOrId("subscription")
				if err != nil {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "collection doesn't exist"})
				}

				// Update Subscription Details
				existingRecord, err := app.FindFirstRecordByData("subscription", "subscription_id", subscription.ID)
				var recordToSave *core.Record

				if err == nil && existingRecord != nil {
					recordToSave = existingRecord
				} else {
					recordToSave = core.NewRecord(collection)
				}

				recordToSave.Set("subscription_id", subscription.ID)
				recordToSave.Set("user_id", uuid)
				recordToSave.Set("metadata", subscription.Metadata)
				recordToSave.Set("status", subscription.Status)
				recordToSave.Set("price_id", subscription.Items.Data[0].Price.ID)
				recordToSave.Set("quantity", subscription.Items.Data[0].Quantity)
				recordToSave.Set("cancel_at_period_end", subscription.CancelAtPeriodEnd)
				recordToSave.Set("cancel_at", int64ToISODate(subscription.CancelAt))
				recordToSave.Set("canceled_at", int64ToISODate(subscription.CanceledAt))
				recordToSave.Set("current_period_start", int64ToISODate(subscription.CurrentPeriodStart))
				recordToSave.Set("current_period_end", int64ToISODate(subscription.CurrentPeriodEnd))
				recordToSave.Set("created", int64ToISODate(subscription.Items.Data[0].Created))
				recordToSave.Set("ended_at", int64ToISODate(subscription.EndedAt))
				recordToSave.Set("trial_start", int64ToISODate(subscription.TrialStart))
				recordToSave.Set("trial_end", int64ToISODate(subscription.TrialEnd))

				if err := app.Save(recordToSave); err != nil {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "couldn't submit subscription update"})
				}

				// Update User Details If Subscription Created
				if event.Type == "customer.subscription.created" {
					existingUserRecord, err := app.FindFirstRecordByData("users", "id", uuid)
					if err == nil && existingUserRecord != nil {
						existingUserRecord.Set("billing_address", subscription.DefaultPaymentMethod.Customer.Address)
						existingUserRecord.Set("payment_method", subscription.DefaultPaymentMethod.Type)

						if err := app.Save(existingUserRecord); err != nil {
							return e.JSON(http.StatusBadRequest, map[string]string{"failure": "couldn't submit user update"})
						}
					}
				}

			case "checkout.session.completed":
				var checkoutSesh stripe.CheckoutSession
				err := json.Unmarshal(event.Data.Raw, &checkoutSesh)
				if err != nil {
					return e.JSON(http.StatusBadRequest, map[string]string{"failure": "failed to marshall the stripe event"})
				}

				if checkoutSesh.Mode == "subscription" {
					// Get customer's UUID from mapping table
					existingCustomer, err := app.FindFirstRecordByData("customer", "stripe_customer_id", checkoutSesh.Subscription.Customer.ID)
					if err != nil {
						return e.JSON(http.StatusBadRequest, map[string]string{"failure": "no customer"})
					}

					uuid := existingCustomer.GetString("user_id")
					collection, err := app.FindCollectionByNameOrId("subscription")
					if err != nil {
						return e.JSON(http.StatusBadRequest, map[string]string{"failure": "collection doesn't exist"})
					}

					// Update Subscription Details
					existingRecord, err := app.FindFirstRecordByData("subscription", "subscription_id", checkoutSesh.Subscription.ID)
					var recordToSave *core.Record

					if err == nil && existingRecord != nil {
						recordToSave = existingRecord
					} else {
						recordToSave = core.NewRecord(collection)
					}

					recordToSave.Set("subscription_id", checkoutSesh.Subscription.ID)
					recordToSave.Set("user_id", uuid)
					recordToSave.Set("metadata", checkoutSesh.Subscription.Metadata)
					recordToSave.Set("status", checkoutSesh.Subscription.Status)
					recordToSave.Set("price_id", checkoutSesh.Subscription.Items.Data[0].Price.ID)
					recordToSave.Set("quantity", checkoutSesh.Subscription.Items.Data[0].Quantity)
					recordToSave.Set("cancel_at_period_end", checkoutSesh.Subscription.CancelAtPeriodEnd)
					recordToSave.Set("cancel_at", int64ToISODate(checkoutSesh.Subscription.CancelAt))
					recordToSave.Set("canceled_at", int64ToISODate(checkoutSesh.Subscription.CanceledAt))
					recordToSave.Set("current_period_start", int64ToISODate(checkoutSesh.Subscription.CurrentPeriodStart))
					recordToSave.Set("current_period_end", int64ToISODate(checkoutSesh.Subscription.CurrentPeriodEnd))
					recordToSave.Set("created", int64ToISODate(checkoutSesh.Subscription.Items.Data[0].Created))
					recordToSave.Set("ended_at", int64ToISODate(checkoutSesh.Subscription.EndedAt))
					recordToSave.Set("trial_start", int64ToISODate(checkoutSesh.Subscription.TrialStart))
					recordToSave.Set("trial_end", int64ToISODate(checkoutSesh.Subscription.TrialEnd))

					if err := app.Save(recordToSave); err != nil {
						return e.JSON(http.StatusBadRequest, map[string]string{"failure": "couldn't submit subscription update"})
					}

					// Update User Details
					existingUserRecord, err := app.FindFirstRecordByData("users", "id", uuid)
					if err == nil && existingUserRecord != nil {
						existingUserRecord.Set("billing_address", checkoutSesh.Subscription.DefaultPaymentMethod.Customer.Address)
						existingUserRecord.Set("payment_method", checkoutSesh.Subscription.DefaultPaymentMethod.Type)

						if err := app.Save(existingUserRecord); err != nil {
							return e.JSON(http.StatusBadRequest, map[string]string{"failure": "couldn't submit user update"})
						}
					}
				}

			default:
				return e.JSON(http.StatusBadRequest, map[string]string{"failure": "didn't receive a valid event"})
			}

			return e.JSON(http.StatusOK, map[string]interface{}{"success": "data was received"})
		})

		return se.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
