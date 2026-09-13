// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelHistoryCmd(flags *rootFlags) *cobra.Command {
	var flagAddressId string
	var flagSince string // reserved for a future local cache; see note below

	cmd := &cobra.Command{
		Use:   "history",
		Short: "See total spend and order counts across Food, Instamart, and Dineout in one view",
		Long: "Use this for a combined view across Food and Instamart.\n" +
			"Do NOT use this for single-domain order lookup; use 'food get-food-orders' or 'instamart get-orders' for that.\n" +
			"Dineout has no orders-list tool (only get-booking-status by id), so it is not included here.",
		Example:     "  swiggy-pp-cli history --address-id addr_01HXYZ --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:novel-scaffold": "true", "pp:happy-args": "--address-id=d6tcokq9681fvjepnc60__AbLNWwSYaOosJZH1CJh5-h"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "history")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			byDomain := map[string]any{}
			totalSpend := 0.0
			totalOrders := 0

			if flagAddressId != "" {
				data, _, err := c.MCPToolQuery(cmd.Context(), "/food", "get_food_orders", nil, map[string]any{"addressId": flagAddressId})
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				orders, err := extractOrdersArray(data)
				if err != nil {
					return fmt.Errorf("parsing food order history: %w", err)
				}
				spend := sumAmounts(orders, "orderTotal")
				byDomain["food"] = map[string]any{"order_count": len(orders), "spend": spend}
				totalSpend += spend
				totalOrders += len(orders)
			} else {
				byDomain["food"] = map[string]any{"skipped": "pass --address-id to include Food orders (get_food_orders requires it)"}
			}

			imData, _, err := c.MCPToolQuery(cmd.Context(), "/im", "get_orders", nil, map[string]any{"count": float64(20)})
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			imOrders, err := extractOrdersArray(imData)
			if err != nil {
				return fmt.Errorf("parsing instamart order history: %w", err)
			}
			imSpend := sumAmounts(imOrders, "totalAmount")
			byDomain["instamart"] = map[string]any{"order_count": len(imOrders), "spend": imSpend}
			totalSpend += imSpend
			totalOrders += len(imOrders)

			byDomain["dineout"] = map[string]any{"skipped": "no bookings-list tool exists on the Dineout MCP server"}

			out := map[string]any{
				"total_spend": totalSpend,
				"order_count": totalOrders,
				"by_domain":   byDomain,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagAddressId, "address-id", "", "Address id to fetch Food order history for (required to include Food; get_food_orders needs it)")
	cmd.Flags().StringVar(&flagSince, "since", "", "Reserved: no local cache is enabled for this CLI (cache.enabled: false), so this filters nothing today")
	return cmd
}

func sumAmounts(orders []map[string]any, key string) float64 {
	total := 0.0
	for _, o := range orders {
		v, ok := o[key]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case float64:
			total += t
		case string:
			if f, err := parseAmountString(t); err == nil {
				total += f
			}
		}
	}
	return total
}

func parseAmountString(s string) (float64, error) {
	cleaned := cleanAmount(s)
	var f float64
	_, err := fmt.Sscanf(cleaned, "%f", &f)
	return f, err
}
