package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/aalvaropc/lynix/internal/infra/curlparse"
	"github.com/aalvaropc/lynix/internal/infra/postmanparse"
	"github.com/aalvaropc/lynix/internal/infra/yamlcollection"
)

func importCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "import",
		Short: "Import collections from external formats",
	}

	c.AddCommand(importCurlCmd())
	c.AddCommand(importPostmanCmd())
	return c
}

func importCurlCmd() *cobra.Command {
	var (
		output   string
		fromFile string
		name     string
	)

	cmd := &cobra.Command{
		Use:   `curl "<command>"`,
		Short: "Import a curl command into a Lynix collection",
		Long:  "Parse a curl command and generate a Lynix YAML collection.\nPass the curl command as a positional argument or use --from-file.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			var input string

			switch {
			case fromFile != "":
				b, err := os.ReadFile(fromFile)
				if err != nil {
					return fmt.Errorf("read --from-file: %w", err)
				}
				input = string(b)
			case len(args) == 1:
				input = args[0]
			default:
				return fmt.Errorf("provide a curl command as argument or use --from-file")
			}

			result, err := curlparse.Parse(input)
			if err != nil {
				return fmt.Errorf("parse curl: %w", err)
			}

			if name != "" {
				result.Collection.Name = name
			}

			b, err := yamlcollection.MarshalCollection(result.Collection)
			if err != nil {
				return fmt.Errorf("marshal collection: %w", err)
			}

			if output != "" {
				if err := os.WriteFile(output, b, 0o644); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
				fmt.Fprintf(os.Stderr, "Collection written to %s\n", output)
			} else {
				fmt.Print(string(b))
			}

			for _, w := range result.Warnings {
				fmt.Fprintf(os.Stderr, "warning: %s\n", w)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Write YAML to file instead of stdout")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Read curl command from a file")
	cmd.Flags().StringVar(&name, "name", "", "Override collection name")
	return cmd
}

func importPostmanCmd() *cobra.Command {
	var (
		output string
		name   string
	)

	cmd := &cobra.Command{
		Use:   "postman <file.json>",
		Short: "Import a Postman v2.1 collection into a Lynix collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			f, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("open postman file: %w", err)
			}
			defer f.Close()

			result, err := postmanparse.Parse(f)
			if err != nil {
				return fmt.Errorf("parse postman: %w", err)
			}

			if name != "" {
				result.Collection.Name = name
			}

			b, err := yamlcollection.MarshalCollection(result.Collection)
			if err != nil {
				return fmt.Errorf("marshal collection: %w", err)
			}

			if output != "" {
				if err := os.WriteFile(output, b, 0o644); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
				fmt.Fprintf(os.Stderr, "Collection written to %s\n", output)
			} else {
				fmt.Print(string(b))
			}

			for _, w := range result.Warnings {
				fmt.Fprintf(os.Stderr, "warning: %s\n", w)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Write YAML to file instead of stdout")
	cmd.Flags().StringVar(&name, "name", "", "Override collection name")
	return cmd
}
