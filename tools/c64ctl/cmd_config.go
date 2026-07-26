package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/c64uploader/go-ultimate"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read and write device settings",
		Long: `Manage Ultimate device configuration. Read current values, change settings,
and persist to flash storage.

Categories and setting names contain spaces — use quotes around them.`,
	}

	cmd.AddCommand(
		newConfigGetCmd(),
		newConfigSetCmd(),
		newConfigSaveCmd(),
		newConfigLoadCmd(),
		newConfigResetCmd(),
	)

	return cmd
}

func newConfigGetCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "get [category] [item]",
		Short: "Read settings from a category, a single item, or all categories",
		Long: `Read device settings.

Without arguments (or with -a) shows all categories.
With a category name, shows all items in that category.
With both category and item, shows that item's value and metadata.`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Single item: category + item
			if len(args) == 2 {
				meta, err := client.Configs.GetItem(ctx, args[0], args[1])
				if err != nil {
					return err
				}
				ci, ok := meta.Get(args[0], args[1])
				if !ok {
					return fmt.Errorf("setting %q / %q not found", args[0], args[1])
				}
				fmt.Printf("%s = %v\n", args[1], formatValue(ci.Current))
				if len(ci.Values) > 0 {
					fmt.Printf("  allowed: %s\n", strings.Join(ci.Values, ", "))
				}
				if ci.Default != nil {
					fmt.Printf("  default: %v\n", ci.Default)
				}
				return nil
			}

			// One category or all categories
			cat := "*"
			if len(args) == 1 {
				cat = args[0]
			} else if !all {
				// No args and no -a → list just the categories (existing 'list' behaviour)
				cats, err := client.Configs.List(ctx)
				if err != nil {
					return err
				}
				for _, c := range cats {
					fmt.Println(c)
				}
				return nil
			}

			cfg, err := client.Configs.Get(ctx, cat)
			if err != nil {
				return err
			}
			// Sort categories for stable output
			catNames := make([]string, 0, len(cfg))
			for c := range cfg {
				catNames = append(catNames, c)
			}
			sort.Strings(catNames)
			for _, c := range catNames {
				fmt.Printf("[%s]\n", c)
				items := cfg[c]
				keys := make([]string, 0, len(items))
				for k := range items {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					fmt.Printf("  %-30s %v\n", k, formatValue(items[k]))
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Show all settings from all categories")
	return cmd
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <category> <item> <value>",
		Short: "Write a setting value",
		Long: `Change a device setting. Use 'config get <category> <item>' first
to see the allowed values and current value.

Changes take effect immediately but are lost on reboot unless
you run 'config save'.`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, item, val := args[0], args[1], args[2]
			if err := client.Configs.Set(context.Background(), cat, item, val); err != nil {
				return err
			}
			fmt.Printf("✓ %s / %s = %s\n", cat, item, val)
			return nil
		},
	}
}

func newConfigSaveCmd() *cobra.Command {
	var category string
	cmd := &cobra.Command{
		Use:   "save [category]",
		Short: "Persist current settings to flash",
		Long: `Save settings to non-volatile flash storage so they survive reboots.
If a category is given, only that category is saved.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := ultimate.ConfigOptions{}
			if len(args) == 1 {
				opts.Category = args[0]
			} else if category != "" {
				opts.Category = category
			}
			if err := client.Configs.SaveToFlash(context.Background(), opts); err != nil {
				return err
			}
			if opts.Category != "" {
				fmt.Printf("✓ Saved %q to flash\n", opts.Category)
			} else {
				fmt.Println("✓ Saved all settings to flash")
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&category, "category", "c", "", "Save only this category")
	return cmd
}

func newConfigLoadCmd() *cobra.Command {
	var category string
	cmd := &cobra.Command{
		Use:   "load [category]",
		Short: "Restore settings from flash",
		Long: `Load settings from flash storage, discarding any runtime changes
made since the last save.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := ultimate.ConfigOptions{}
			if len(args) == 1 {
				opts.Category = args[0]
			} else if category != "" {
				opts.Category = category
			}
			if err := client.Configs.LoadFromFlash(context.Background(), opts); err != nil {
				return err
			}
			if opts.Category != "" {
				fmt.Printf("✓ Loaded %q from flash\n", opts.Category)
			} else {
				fmt.Println("✓ Loaded all settings from flash")
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&category, "category", "c", "", "Load only this category")
	return cmd
}

func newConfigResetCmd() *cobra.Command {
	var category string
	cmd := &cobra.Command{
		Use:   "reset [category]",
		Short: "Reset settings to factory defaults",
		Long: `Reset settings to factory defaults. Changes are not persisted
until you run 'config save'.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := ultimate.ConfigOptions{}
			if len(args) == 1 {
				opts.Category = args[0]
			} else if category != "" {
				opts.Category = category
			}
			if err := client.Configs.ResetToDefault(context.Background(), opts); err != nil {
				return err
			}
			if opts.Category != "" {
				fmt.Printf("✓ Reset %q to defaults (run 'config save' to persist)\n", opts.Category)
			} else {
				fmt.Println("✓ Reset all settings to defaults (run 'config save' to persist)")
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&category, "category", "c", "", "Reset only this category")
	return cmd
}

func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%v", val), "0"), ".")
	default:
		return fmt.Sprint(v)
	}
}
