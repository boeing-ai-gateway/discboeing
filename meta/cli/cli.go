// Package cli implements the metactl command-line interface.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/obot-platform/discobot/meta/client"
)

type options struct {
	baseURL string
	token   string
	out     io.Writer
	errOut  io.Writer
}

// New returns the root metactl command.
func New(out, errOut io.Writer) *cobra.Command {
	if out == nil {
		out = os.Stdout
	}
	if errOut == nil {
		errOut = os.Stderr
	}
	opts := &options{
		baseURL: envDefault("META_URL", "http://127.0.0.1:3011"),
		token:   os.Getenv("META_TOKEN"),
		out:     out,
		errOut:  errOut,
	}

	cmd := &cobra.Command{
		Use:   "metactl",
		Short: "Minimal CLI for the Discobot Meta API",
		Long: `metactl is a small CLI for interacting with the Discobot Meta API.

Most service endpoints are still under construction, so this CLI intentionally
keeps a generic raw request command alongside a few convenience commands.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.PersistentFlags().StringVar(&opts.baseURL, "url", opts.baseURL, "Meta base URL")
	cmd.PersistentFlags().StringVar(&opts.token, "token", opts.token, "Bearer token")

	cmd.AddCommand(
		newWhoamiCommand(opts),
		newOpenIDCommand(opts),
		newJWKSCommand(opts),
		newOrganizationsCommand(opts),
		newProjectsCommand(opts),
		newOAuthCommand(opts),
		newApplyCommand(opts),
		newRoutesCommand(opts),
		newRawCommand(opts),
	)
	return cmd
}

// Execute runs the root metactl command.
func Execute(args []string, out, errOut io.Writer) error {
	cmd := New(out, errOut)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func newWhoamiCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the current authenticated principal",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := commandContext()
			defer cancel()
			resp, err := opts.client().Whoami(ctx, client.WhoamiParams{})
			return printResponse(opts.out, resp, err)
		},
	}
}

func newOpenIDCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "openid",
		Short: "Fetch OIDC discovery metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := commandContext()
			defer cancel()
			resp, err := opts.client().GetOpenIDConfiguration(ctx, client.GetOpenIDConfigurationParams{})
			return printResponse(opts.out, resp, err)
		},
	}
}

func newJWKSCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "jwks",
		Short: "Fetch JWKS",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := commandContext()
			defer cancel()
			resp, err := opts.client().GetJWKS(ctx, client.GetJWKSParams{})
			return printResponse(opts.out, resp, err)
		},
	}
}

func newOrganizationsCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:     "orgs",
		Aliases: []string{"organizations"},
		Short:   "List organizations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := commandContext()
			defer cancel()
			resp, err := opts.client().ListOrganizations(ctx, client.ListOrganizationsParams{})
			return printResponse(opts.out, resp, err)
		},
	}
}

func newProjectsCommand(opts *options) *cobra.Command {
	var org string
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "List projects",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := commandContext()
			defer cancel()
			if org == "" {
				resp, err := opts.client().ListProjects(ctx, client.ListProjectsParams{})
				return printResponse(opts.out, resp, err)
			}
			resp, err := opts.client().ListOrganizationProjects(ctx, client.ListOrganizationProjectsParams{OrganizationDomain: org})
			return printResponse(opts.out, resp, err)
		},
	}
	cmd.Flags().StringVar(&org, "org", "", "organization domain; omit for public shortcut")
	return cmd
}

func newOAuthCommand(opts *options) *cobra.Command {
	var org string
	var output string
	cmd := &cobra.Command{
		Use:     "oauth",
		Aliases: []string{"oauth-apps", "oauth-applications"},
		Short:   "Manage organization OAuth applications",
	}
	cmd.PersistentFlags().StringVar(&org, "org", "", "organization domain")
	cmd.PersistentFlags().StringVarP(&output, "output", "o", "table", "output format: table, json, yaml")

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List OAuth applications",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if org == "" {
				return fmt.Errorf("--org is required")
			}
			ctx, cancel := commandContext()
			defer cancel()
			resp, err := opts.client().ListOrganizationOAuthApplications(ctx, client.ListOrganizationOAuthApplicationsParams{OrganizationDomain: org})
			return printTypedResponse(opts.out, resp, err, resourceTypeOAuthApplication, output)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get ID",
		Short: "Get one OAuth application",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if org == "" {
				return fmt.Errorf("--org is required")
			}
			ctx, cancel := commandContext()
			defer cancel()
			resp, err := opts.client().GetOrganizationOAuthApplication(ctx, client.GetOrganizationOAuthApplicationParams{
				OrganizationDomain: org,
				OAuthApplicationID: args[0],
			})
			return printTypedResponse(opts.out, resp, err, resourceTypeOAuthApplication, output)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "delete ID",
		Short: "Delete one OAuth application",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if org == "" {
				return fmt.Errorf("--org is required")
			}
			ctx, cancel := commandContext()
			defer cancel()
			resp, err := opts.client().DeleteOrganizationOAuthApplication(ctx, client.DeleteOrganizationOAuthApplicationParams{
				OrganizationDomain: org,
				OAuthApplicationID: args[0],
			})
			return printResponse(opts.out, resp, err)
		},
	})

	return cmd
}

func newApplyCommand(opts *options) *cobra.Command {
	var file string
	var org string
	cmd := &cobra.Command{
		Use:   "apply -f FILE",
		Short: "Apply declarative Meta resources",
		Example: `  metactl apply -f oauth.yaml

  cat <<EOF | metactl apply -f -
  type: OAuthApplication
  name: github
  organization: example.com
  provider: github
  clientId: github-client-id
  clientSecret: github-client-secret
  redirectUris:
    - https://meta.example.com/oauth/github/callback
  EOF`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if file == "" {
				return fmt.Errorf("-f is required")
			}
			resources, err := readApplyResources(file)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext()
			defer cancel()
			for _, resource := range resources {
				if err := opts.applyResource(ctx, resource.withOrganization(org)); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "filename", "f", "", "resource file to apply, or - for stdin")
	cmd.Flags().StringVar(&org, "org", "", "organization domain override")
	return cmd
}

func newRoutesCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "routes",
		Short: "List generated route metadata from /api/routes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := commandContext()
			defer cancel()
			resp, err := opts.client().Do(ctx, http.MethodGet, "/api/routes", nil)
			return printResponse(opts.out, resp, err)
		},
	}
}

func newRawCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "raw METHOD PATH [JSON_BODY]",
		Short: "Send a raw request for testing",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			path := args[1]
			var requestOpts []client.RequestOption
			if len(args) > 2 {
				requestOpts = append(requestOpts, client.WithBody("application/json", strings.NewReader(args[2])))
			}
			ctx, cancel := commandContext()
			defer cancel()
			resp, err := opts.client().Do(ctx, method, path, url.Values{}, requestOpts...)
			return printResponse(opts.out, resp, err)
		},
	}
}

func (o *options) client() *client.Client {
	c := client.New(o.baseURL)
	c.BearerToken = o.token
	return c
}

func commandContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

func printResponse(w io.Writer, resp *client.Response, err error) error {
	if resp != nil {
		_, _ = fmt.Fprintf(w, "status: %d\n", resp.StatusCode)
		if len(resp.Body) > 0 {
			var value any
			if json.Unmarshal(resp.Body, &value) == nil {
				formatted, _ := json.MarshalIndent(value, "", "  ")
				_, _ = fmt.Fprintln(w, string(formatted))
			} else {
				_, _ = fmt.Fprintln(w, string(resp.Body))
			}
		}
	}
	if err != nil {
		return err
	}
	return nil
}

const resourceTypeOAuthApplication = "OAuthApplication"

type outputFormat string

const (
	outputTable outputFormat = "table"
	outputJSON  outputFormat = "json"
	outputYAML  outputFormat = "yaml"
)

type tablePrinter struct {
	columns []string
	row     func(map[string]any) []string
}

var tablePrinters = map[string]tablePrinter{
	resourceTypeOAuthApplication: {
		columns: []string{"ID", "NAME", "PROVIDER", "CLIENT ID", "SECRET", "STATUS"},
		row: func(item map[string]any) []string {
			return []string{
				stringValue(item["id"]),
				stringValue(item["name"]),
				stringValue(item["provider"]),
				stringValue(item["clientId"]),
				boolString(item["hasClientSecret"]),
				stringValue(item["status"]),
			}
		},
	},
}

func printTypedResponse(w io.Writer, resp *client.Response, err error, resourceType, format string) error {
	if err != nil {
		return printResponse(w, resp, err)
	}
	if resp == nil || len(resp.Body) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(resp.Body, &value); err != nil {
		return err
	}
	switch outputFormat(strings.ToLower(format)) {
	case "", outputTable:
		return printTable(w, resourceType, value)
	case outputJSON:
		formatted, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(w, string(formatted))
	case outputYAML:
		formatted, err := yaml.Marshal(value)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprint(w, string(formatted))
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
	return nil
}

func printTable(w io.Writer, resourceType string, value any) error {
	printer, ok := tablePrinters[resourceType]
	if !ok {
		return fmt.Errorf("no table printer registered for %s", resourceType)
	}
	rows := tableRows(value)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, strings.Join(printer.columns, "\t"))
	for _, row := range rows {
		_, _ = fmt.Fprintln(tw, strings.Join(printer.row(row), "\t"))
	}
	return tw.Flush()
}

func tableRows(value any) []map[string]any {
	if object, ok := value.(map[string]any); ok {
		if items, ok := object["items"].([]any); ok {
			rows := make([]map[string]any, 0, len(items))
			for _, item := range items {
				if row, ok := item.(map[string]any); ok {
					rows = append(rows, row)
				}
			}
			return rows
		}
		return []map[string]any{object}
	}
	return nil
}

func stringValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func boolString(value any) string {
	if b, ok := value.(bool); ok {
		if b {
			return "yes"
		}
		return "no"
	}
	return stringValue(value)
}

type applyResource map[string]any

func readApplyResources(file string) ([]applyResource, error) {
	var data []byte
	var err error
	if file == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(file)
	}
	if err != nil {
		return nil, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var resources []applyResource
	for {
		var resource applyResource
		err := decoder.Decode(&resource)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if resourceString(resource, "type") == "" {
			continue
		}
		resources = append(resources, resource)
	}
	if len(resources) == 0 {
		return nil, fmt.Errorf("no resources found in %s", file)
	}
	return resources, nil
}

func (o *options) applyResource(ctx context.Context, resource applyResource) error {
	resourceType := resourceString(resource, "type")
	result, err := o.client().Apply(ctx, resourceType, resource)
	if err != nil {
		if result != nil && result.Response != nil {
			return printResponse(o.out, result.Response, err)
		}
		return err
	}
	_, _ = fmt.Fprintf(o.out, "%s/%s %s\n", strings.ToLower(result.Type), result.Name, result.Operation)
	return nil
}

func resourceString(resource applyResource, key string) string {
	if value, ok := resource[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func (resource applyResource) withOrganization(org string) applyResource {
	if org == "" {
		return resource
	}
	copy := make(applyResource, len(resource)+1)
	for key, value := range resource {
		copy[key] = value
	}
	copy["organization"] = org
	return copy
}

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
