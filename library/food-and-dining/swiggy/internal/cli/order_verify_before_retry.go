// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelOrderVerifyBeforeRetryCmd(flags *rootFlags) *cobra.Command {
	var flagDomain string
	var flagAddressId string
	var flagRestaurantId string
	var flagAmount string
	var flagRecent int

	cmd := &cobra.Command{
		Use:   "verify-before-retry",
		Short: "Check whether a food or grocery order actually went through before retrying a failed placement.",
		Long: "Use this before re-issuing place-food-order/checkout after a 5xx or timeout.\n" +
			"place_food_order and checkout are explicitly documented as not idempotent.\n" +
			"Do NOT use this as a substitute for get-food-order-details, which looks up a known order ID directly.",
		Example:     "  swiggy-pp-cli order verify-before-retry --domain food --address-id addr_01HXYZ --restaurant-id r_123 --amount 450",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:novel-scaffold": "true", "pp:happy-args": "--domain=food;--address-id=d6tcokq9681fvjepnc60__AbLNWwSYaOosJZH1CJh5-h;--amount=450"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "order verify-before-retry")
			}
			if flagAmount == "" {
				return fmt.Errorf("required flag \"amount\" not set")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var orders []map[string]any
			switch flagDomain {
			case "food":
				if flagAddressId == "" {
					return fmt.Errorf("--address-id is required for --domain food (get_food_orders requires it)")
				}
				data, _, err := c.MCPToolQuery(cmd.Context(), "/food", "get_food_orders", nil, map[string]any{"addressId": flagAddressId})
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				orders, err = extractOrdersArray(data)
				if err != nil {
					// This is the safety-critical path: silently treating an
					// unparseable order-history response as "zero orders"
					// would report "safe to retry" even though we never
					// actually confirmed the prior order didn't go through.
					// Fail loudly instead of guessing.
					return fmt.Errorf("could not verify order history before retrying (order history response was unparseable, so it is not safe to assume the previous order did not go through): %w", err)
				}
			case "instamart":
				data, _, err := c.MCPToolQuery(cmd.Context(), "/im", "get_orders", nil, map[string]any{"count": float64(20)})
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				orders, err = extractOrdersArray(data)
				if err != nil {
					return fmt.Errorf("could not verify order history before retrying (order history response was unparseable, so it is not safe to assume the previous order did not go through): %w", err)
				}
			case "dineout":
				return fmt.Errorf("dineout has no orders-list tool to reconcile against; use 'dineout get-booking-status --booking-id <id>' directly with a known booking id")
			default:
				return fmt.Errorf("invalid --domain %q; must be one of food, instamart", flagDomain)
			}

			if flagRecent <= 0 {
				flagRecent = 10
			}
			if len(orders) > flagRecent {
				orders = orders[:flagRecent]
			}

			var match map[string]any
			for _, o := range orders {
				if !amountMatches(o, flagAmount) {
					continue
				}
				if flagRestaurantId != "" {
					rid, _ := o["restaurantId"].(string)
					if rid != flagRestaurantId {
						continue
					}
				}
				match = o
				break
			}

			w := cmd.OutOrStdout()
			if flags.asJSON {
				out := map[string]any{
					"placed":         match != nil,
					"matched_order":  match,
					"checked_orders": len(orders),
				}
				return printJSONFiltered(w, out, flags)
			}
			if match != nil {
				fmt.Fprintln(w, green("Already placed — do NOT retry."))
				if id, ok := match["orderId"].(string); ok {
					fmt.Fprintf(w, "  Matching order id: %s\n", id)
				}
				return nil
			}
			fmt.Fprintln(w, "No matching recent order found — safe to retry the placement call.")
			fmt.Fprintf(w, "  (checked the %d most recent orders for amount ~= %s%s)\n", len(orders), flagAmount,
				func() string {
					if flagRestaurantId != "" {
						return " and restaurant " + flagRestaurantId
					}
					return ""
				}())
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDomain, "domain", "", "One of food, instamart")
	cmd.Flags().StringVar(&flagAddressId, "address-id", "", "Required for --domain food")
	cmd.Flags().StringVar(&flagRestaurantId, "restaurant-id", "", "Optional: narrow the match to this restaurant (food only)")
	cmd.Flags().StringVar(&flagAmount, "amount", "", "The order total you expected to be charged, e.g. 450")
	cmd.Flags().IntVar(&flagRecent, "recent", 10, "How many of the most recent orders to check")
	return cmd
}

// extractOrdersArray pulls the orders list out of either get_food_orders'
// {data:{orders:[...]}} or get_orders' {data:{orders:[...]}} shape without
// assuming a strict schema, since the exact field set differs between Food
// and Instamart (see the research brief's Codebase Intelligence section).
//
// Returns an error rather than silently swallowing a parse failure: callers
// on the safety-critical verify-before-retry path must not mistake "we
// couldn't parse the order history" for "we checked and found zero orders",
// since the latter is reported to the user as "safe to retry".
func extractOrdersArray(raw json.RawMessage) ([]map[string]any, error) {
	var envelope struct {
		Data struct {
			Orders []map[string]any `json:"orders"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("parsing orders response: %w", err)
	}
	return envelope.Data.Orders, nil
}

// amountMatches compares a user-supplied amount string against whichever
// amount-shaped field the order carries (orderTotal for Food, often a string
// with a currency symbol; totalAmount for Instamart, numeric). Currency
// symbols and whitespace are stripped before comparing as floats.
func amountMatches(order map[string]any, want string) bool {
	wantF, err := strconv.ParseFloat(cleanAmount(want), 64)
	if err != nil {
		return false
	}
	for _, key := range []string{"orderTotal", "totalAmount", "amount"} {
		v, ok := order[key]
		if !ok {
			continue
		}
		var haveF float64
		switch t := v.(type) {
		case float64:
			haveF = t
		case string:
			f, err := strconv.ParseFloat(cleanAmount(t), 64)
			if err != nil {
				continue
			}
			haveF = f
		default:
			continue
		}
		// A tolerance instead of exact equality: floating-point amounts
		// parsed from JSON/currency strings can differ by fractions of a
		// paisa from rounding without being a genuinely different order.
		// Exact "==" is the dangerous direction on this path — a false
		// negative here reports "safe to retry" for an order that actually
		// did go through.
		const amountEpsilon = 0.01
		if diff := haveF - wantF; diff > -amountEpsilon && diff < amountEpsilon {
			return true
		}
	}
	return false
}

func cleanAmount(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "₹")
	s = strings.ReplaceAll(s, ",", "")
	return strings.TrimSpace(s)
}
