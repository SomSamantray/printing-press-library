// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source computed

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/swiggy/internal/config"
	"github.com/spf13/cobra"
)

func newNovelStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "See at a glance whether your Swiggy login is still valid and which domain sessions are active.",
		Long: "Use this to check whether you need to re-run 'auth login' before starting a session.\n" +
			"Do NOT use this to perform login itself; it is read-only. Run 'auth login' to authenticate.",
		Example:     "  swiggy-pp-cli status",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:novel-scaffold": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "status")
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			authenticated := cfg.AccessToken != ""
			var expiresIn string
			var expired bool
			if authenticated && !cfg.TokenExpiry.IsZero() {
				remaining := time.Until(cfg.TokenExpiry)
				expired = remaining <= 0
				if expired {
					expiresIn = "expired"
				} else {
					expiresIn = remaining.Round(time.Minute).String()
				}
			}

			w := cmd.OutOrStdout()
			if flags.asJSON {
				out := map[string]any{
					"authenticated":      authenticated,
					"token_expired":      expired,
					"token_expires_in":   expiresIn,
					"token_expiry_utc":   cfg.TokenExpiry.UTC().Format(time.RFC3339),
					"config_path":        cfg.Path,
					"reauth_recommended": !authenticated || expired,
				}
				return printJSONFiltered(w, out, flags)
			}

			if !authenticated {
				fmt.Fprintln(w, red("Not authenticated"))
				fmt.Fprintln(w, "  Run 'swiggy-pp-cli auth login' to complete the OAuth 2.1 browser login (phone + OTP).")
				return authErr(fmt.Errorf("no Swiggy access token stored"))
			}
			if expired {
				fmt.Fprintln(w, red("Access token expired"))
				fmt.Fprintln(w, "  Swiggy access tokens last 5 days with no refresh-token issuance in v1.0.")
				fmt.Fprintln(w, "  Run 'swiggy-pp-cli auth login' to re-authenticate.")
				return authErr(fmt.Errorf("access token expired at %s", cfg.TokenExpiry.UTC().Format(time.RFC3339)))
			}
			fmt.Fprintln(w, green("Authenticated"))
			fmt.Fprintf(w, "  Token expires in: %s (%s)\n", expiresIn, cfg.TokenExpiry.UTC().Format(time.RFC3339))
			fmt.Fprintln(w, "  Domains share this one session token: food, instamart, dineout each POST to their own endpoint under it.")
			return nil
		},
	}
	return cmd
}
