package role

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/sensu/sensu-go/cli"
	"github.com/sensu/sensu-go/cli/commands/helpers"
	"github.com/sensu/sensu-go/cli/elements/table"
	"github.com/sensu/sensu-go/types"
	"github.com/spf13/cobra"
)

// InfoCommand defines new command to display detailed information about a role
func InfoCommand(cli *cli.SensuCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "info [NAME]",
		Aliases:      []string{"list-rules"}, // backward compatibility
		Short:        "show detailed information about a role",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				_ = cmd.Help()
				return errors.New("a role name is required")
			}

			// Fetch roles from API
			r, err := cli.Client.FetchRole(args[0])
			if err != nil {
				return err
			}

			// Determine the format to use to output the data
			flag := helpers.GetChangedStringValueViper("format", cmd.Flags())
			format := cli.Config.Format()
			return helpers.PrintFormatted(flag, format, r, cmd.OutOrStdout(), printRulesToTable)
		},
	}

	helpers.AddFormatFlag(cmd.Flags())

	return cmd
}

func printRulesToTable(v interface{}, io io.Writer) error {
	queryResults, ok := v.(*types.Role)
	if !ok {
		return fmt.Errorf("%t is not a role", v)
	}
	table := table.New([]*table.Column{
		{
			Title:       "Namespace",
			ColumnStyle: table.PrimaryTextStyle,
			CellTransformer: func(data interface{}) string {
				return queryResults.Namespace
			},
		},
		{
			Title: "Verbs",
			CellTransformer: func(data interface{}) string {
				rule, ok := data.(types.Rule)
				if !ok {
					return cli.TypeError
				}
				return strings.Join(rule.Verbs, ",")
			},
		},
		{
			Title: "Resources",
			CellTransformer: func(data interface{}) string {
				rule, ok := data.(types.Rule)
				if !ok {
					return cli.TypeError
				}
				return strings.Join(rule.Resources, ",")
			},
		},
		{
			Title: "Resource Names",
			CellTransformer: func(data interface{}) string {
				rule, ok := data.(types.Rule)
				if !ok {
					return cli.TypeError
				}
				return strings.Join(rule.ResourceNames, ",")
			},
		},
	})

	table.Render(io, queryResults.Rules)
	return nil
}
