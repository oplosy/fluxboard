package stripex

import (
	"context"
	"errors"
	"fmt"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/mesutokul/fluxboard/backend/internal/domain/billing"
)

// CatalogPlan is a paid plan as the Stripe catalog needs it (cmd/stripeseed).
type CatalogPlan struct {
	Code           string
	Name           string
	SeatUnitAmount int64 // per seat per month, minor units
	Metered        bool  // adds the storage and API-call usage prices
}

// CatalogIDs are the Stripe ids stripe-seed writes into the plans table.
type CatalogIDs struct {
	ProductID      string
	SeatPriceID    string
	StoragePriceID string // "" unless metered
	APIPriceID     string // "" unless metered
}

// CatalogVersion (billing.CatalogVersion) is part of every price lookup key.
// Prices are immutable in Stripe: changing an amount means bumping it and
// re-seeding, which creates new prices next to the old ones (existing
// subscriptions keep theirs; their plan is still found by product id).
const CatalogVersion = billing.CatalogVersion

// Usage prices are $0 until the plan catalog (report B02) sets real amounts:
// meters still record every customer's usage and invoices show the lines.
const usageUnitAmount = 0

// SeedCatalog makes the Stripe side of the plan catalog exist, idempotently:
// one Billing Meter per usage dimension, one product per paid plan (fixed id
// "fluxboard_<code>"), a licensed monthly seat price and, for metered plans,
// usage prices on the meters. Prices are found by lookup key; an existing price
// whose amount, currency or interval differs from the catalog is an error, not
// a silent mismatch.
func SeedCatalog(ctx context.Context, sc *stripe.Client, currency string, plans []CatalogPlan) (map[string]CatalogIDs, error) {
	meters := map[string]string{} // event name → meter id
	for event, formula := range map[string]string{MeterStorageBytes: "last", MeterAPICalls: "sum"} {
		id, err := ensureMeter(ctx, sc, event, formula)
		if err != nil {
			return nil, err
		}
		meters[event] = id
	}

	out := make(map[string]CatalogIDs, len(plans))
	for _, p := range plans {
		if p.SeatUnitAmount <= 0 {
			return nil, fmt.Errorf("stripe catalog: plan %s needs a positive seat price", p.Code)
		}
		productID, err := ensureProduct(ctx, sc, ProductID(p.Code), "Fluxboard "+p.Name)
		if err != nil {
			return nil, err
		}
		ids := CatalogIDs{ProductID: productID}
		ids.SeatPriceID, err = ensurePrice(ctx, sc, priceSpec{
			lookupKey: fmt.Sprintf("fluxboard_%s_seat_%s", p.Code, CatalogVersion),
			product:   productID, currency: currency, unitAmount: p.SeatUnitAmount,
			usageType: "licensed",
		})
		if err != nil {
			return nil, err
		}
		if p.Metered {
			ids.StoragePriceID, err = ensurePrice(ctx, sc, priceSpec{
				lookupKey: fmt.Sprintf("fluxboard_%s_storage_%s", p.Code, CatalogVersion),
				product:   productID, currency: currency, unitAmount: usageUnitAmount,
				usageType: "metered", meter: meters[MeterStorageBytes],
			})
			if err != nil {
				return nil, err
			}
			ids.APIPriceID, err = ensurePrice(ctx, sc, priceSpec{
				lookupKey: fmt.Sprintf("fluxboard_%s_api_calls_%s", p.Code, CatalogVersion),
				product:   productID, currency: currency, unitAmount: usageUnitAmount,
				usageType: "metered", meter: meters[MeterAPICalls],
			})
			if err != nil {
				return nil, err
			}
		}
		out[p.Code] = ids
	}
	return out, nil
}

func ensureMeter(ctx context.Context, sc *stripe.Client, event, formula string) (string, error) {
	for m, err := range sc.V1BillingMeters.List(ctx, &stripe.BillingMeterListParams{Status: stripe.String("active")}).All(ctx) {
		if err != nil {
			return "", fmt.Errorf("stripe catalog: list meters: %w", err)
		}
		if m.EventName == event {
			if string(m.DefaultAggregation.Formula) != formula {
				return "", fmt.Errorf("stripe catalog: meter %s aggregates with %q, want %q", event, m.DefaultAggregation.Formula, formula)
			}
			return m.ID, nil
		}
	}
	m, err := sc.V1BillingMeters.Create(ctx, &stripe.BillingMeterCreateParams{
		DisplayName: stripe.String(event),
		EventName:   stripe.String(event),
		DefaultAggregation: &stripe.BillingMeterCreateDefaultAggregationParams{
			Formula: stripe.String(formula),
		},
		CustomerMapping: &stripe.BillingMeterCreateCustomerMappingParams{
			Type:            stripe.String("by_id"),
			EventPayloadKey: stripe.String("stripe_customer_id"),
		},
		ValueSettings: &stripe.BillingMeterCreateValueSettingsParams{
			EventPayloadKey: stripe.String("value"),
		},
	})
	if err != nil {
		return "", fmt.Errorf("stripe catalog: create meter %s: %w", event, err)
	}
	return m.ID, nil
}

func ensureProduct(ctx context.Context, sc *stripe.Client, id, name string) (string, error) {
	p, err := sc.V1Products.Retrieve(ctx, id, &stripe.ProductRetrieveParams{})
	if err == nil {
		return p.ID, nil
	}
	var se *stripe.Error
	if !errors.As(err, &se) || se.Code != stripe.ErrorCodeResourceMissing {
		return "", fmt.Errorf("stripe catalog: retrieve product %s: %w", id, err)
	}
	p, err = sc.V1Products.Create(ctx, &stripe.ProductCreateParams{ID: stripe.String(id), Name: stripe.String(name)})
	if err != nil {
		return "", fmt.Errorf("stripe catalog: create product %s: %w", id, err)
	}
	return p.ID, nil
}

type priceSpec struct {
	lookupKey, product, currency string
	unitAmount                   int64
	usageType                    string // licensed | metered
	meter                        string // metered only
}

func ensurePrice(ctx context.Context, sc *stripe.Client, s priceSpec) (string, error) {
	list := sc.V1Prices.List(ctx, &stripe.PriceListParams{
		LookupKeys: []*string{stripe.String(s.lookupKey)},
		Active:     stripe.Bool(true),
	})
	for p, err := range list.All(ctx) {
		if err != nil {
			return "", fmt.Errorf("stripe catalog: list price %s: %w", s.lookupKey, err)
		}
		if err := s.matches(p); err != nil {
			return "", fmt.Errorf("stripe catalog: price %s (%s) differs from the catalog: %w; bump CatalogVersion to create a new price", s.lookupKey, p.ID, err)
		}
		return p.ID, nil
	}
	recurring := &stripe.PriceCreateRecurringParams{
		Interval:  stripe.String("month"),
		UsageType: stripe.String(s.usageType),
	}
	if s.meter != "" {
		recurring.Meter = stripe.String(s.meter)
	}
	p, err := sc.V1Prices.Create(ctx, &stripe.PriceCreateParams{
		Product:    stripe.String(s.product),
		Currency:   stripe.String(s.currency),
		UnitAmount: stripe.Int64(s.unitAmount),
		Recurring:  recurring,
		LookupKey:  stripe.String(s.lookupKey),
	})
	if err != nil {
		return "", fmt.Errorf("stripe catalog: create price %s: %w", s.lookupKey, err)
	}
	return p.ID, nil
}

func (s priceSpec) matches(p *stripe.Price) error {
	switch {
	case string(p.Currency) != s.currency:
		return fmt.Errorf("currency %s, want %s", p.Currency, s.currency)
	case p.UnitAmount != s.unitAmount:
		return fmt.Errorf("unit amount %d, want %d", p.UnitAmount, s.unitAmount)
	case p.Recurring == nil || p.Recurring.Interval != "month":
		return errors.New("not a monthly recurring price")
	case string(p.Recurring.UsageType) != s.usageType:
		return fmt.Errorf("usage type %s, want %s", p.Recurring.UsageType, s.usageType)
	case s.meter != "" && p.Recurring.Meter != s.meter:
		return fmt.Errorf("meter %s, want %s", p.Recurring.Meter, s.meter)
	}
	return nil
}
