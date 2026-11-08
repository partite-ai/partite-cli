// Package modelprovider manages model providers: the stored credentials for
// a model service such as Anthropic or OpenAI.
package modelprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/partite-ai/partite-cli/internal/cmdutil"
	gql "github.com/partite-ai/partite-cli/internal/graphql"
	"github.com/partite-ai/partite-cli/internal/output"
	"github.com/spf13/cobra"
)

// Provider is the view printed by get and read by -f. Exactly one of the
// service blocks is set. Secret fields are only read; get never fills them and
// shows the *_set fields instead.
type Provider struct {
	ID               string                  `json:"id,omitempty"`
	Name             string                  `json:"name"`
	Description      string                  `json:"description,omitempty"`
	Anthropic        *KeyConfig              `json:"anthropic,omitempty"`
	OpenAI           *KeyConfig              `json:"openai,omitempty"`
	Cerebras         *KeyConfig              `json:"cerebras,omitempty"`
	Fireworks        *FireworksConfig        `json:"fireworks,omitempty"`
	OpenAICompatible *OpenAICompatibleConfig `json:"openai_compatible,omitempty"`
	GoogleGenAI      *GoogleGenAIConfig      `json:"google_genai,omitempty"`
	// Probe asks the server to list the provider's models with new credentials
	// before saving them, and to refuse credentials the provider rejects.
	Probe     bool          `json:"probe,omitempty"`
	Tags      []cmdutil.Tag `json:"tags"`
	CreatedAt string        `json:"created_at,omitempty"`
	UpdatedAt string        `json:"updated_at,omitempty"`
}

type KeyConfig struct {
	APIKey    string `json:"api_key,omitempty"`
	APIKeyEnv string `json:"api_key_env,omitempty"`
	APIKeySet bool   `json:"api_key_set,omitempty"`
}

type FireworksConfig struct {
	KeyConfig
	AccountID string `json:"account_id,omitempty"`
}

type OpenAICompatibleConfig struct {
	URL         string `json:"url"`
	APIToken    string `json:"api_token,omitempty"`
	APITokenEnv string `json:"api_token_env,omitempty"`
	APITokenSet bool   `json:"api_token_set,omitempty"`
}

type GoogleGenAIConfig struct {
	Gemini *KeyConfig `json:"gemini,omitempty"`
}

// Kinds of provider, as the --type flag names them.
const (
	kindAnthropic        = "anthropic"
	kindOpenAI           = "openai"
	kindCerebras         = "cerebras"
	kindFireworks        = "fireworks"
	kindOpenAICompatible = "openai-compatible"
	kindGemini           = "gemini"
)

var kinds = []string{kindAnthropic, kindOpenAI, kindOpenAICompatible, kindFireworks, kindCerebras, kindGemini}

var kindTypes = map[string]gql.ModelProviderType{
	kindAnthropic:        gql.ModelProviderTypeAnthropic,
	kindOpenAI:           gql.ModelProviderTypeOpenAi,
	kindCerebras:         gql.ModelProviderTypeCerebras,
	kindFireworks:        gql.ModelProviderTypeFireworks,
	kindOpenAICompatible: gql.ModelProviderTypeOpenAiCompatible,
	kindGemini:           gql.ModelProviderTypeGoogleGenAi,
}

// TypeName is the short name of a provider type, such as openai-compatible.
func TypeName(t gql.ModelProviderType) string {
	if t == gql.ModelProviderTypeGoogleGenAi {
		return "google-genai"
	}
	for k, v := range kindTypes {
		if v == t {
			return k
		}
	}
	return strings.ToLower(string(t))
}

func normalizeKind(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "_", "-")
}

// kind returns which service block is set.
func (p *Provider) kind() (string, error) {
	var set []string
	if p.Anthropic != nil {
		set = append(set, kindAnthropic)
	}
	if p.OpenAI != nil {
		set = append(set, kindOpenAI)
	}
	if p.Cerebras != nil {
		set = append(set, kindCerebras)
	}
	if p.Fireworks != nil {
		set = append(set, kindFireworks)
	}
	if p.OpenAICompatible != nil {
		set = append(set, kindOpenAICompatible)
	}
	if g := p.GoogleGenAI; g != nil {
		if g.Gemini == nil {
			return "", cmdutil.UsageError("google_genai needs a gemini block")
		}
		set = append(set, kindGemini)
	}
	switch len(set) {
	case 0:
		return "", cmdutil.UsageError("no provider type; pass --type, or set one of anthropic, openai, openai_compatible, fireworks, cerebras or google_genai")
	case 1:
		return set[0], nil
	}
	return "", cmdutil.UsageError("a provider has one type, but %s are all set", strings.Join(set, ", "))
}

// keyConfig returns the block that holds an api_key, for the kinds that have one.
func (p *Provider) keyConfig(kind string) *KeyConfig {
	switch kind {
	case kindAnthropic:
		return p.Anthropic
	case kindOpenAI:
		return p.OpenAI
	case kindCerebras:
		return p.Cerebras
	case kindFireworks:
		return &p.Fireworks.KeyConfig
	case kindGemini:
		return p.GoogleGenAI.Gemini
	}
	return nil
}

// setKind adds an empty block for kind.
func (p *Provider) setKind(kind string) {
	switch kind {
	case kindAnthropic:
		p.Anthropic = &KeyConfig{}
	case kindOpenAI:
		p.OpenAI = &KeyConfig{}
	case kindCerebras:
		p.Cerebras = &KeyConfig{}
	case kindFireworks:
		p.Fireworks = &FireworksConfig{}
	case kindOpenAICompatible:
		p.OpenAICompatible = &OpenAICompatibleConfig{}
	case kindGemini:
		p.GoogleGenAI = &GoogleGenAIConfig{Gemini: &KeyConfig{}}
	}
}

// settings is every setting that is not a secret, for comparing two providers.
func (p *Provider) settings() string {
	k, _ := p.kind()
	s := k
	if p.Fireworks != nil {
		s += "|" + p.Fireworks.AccountID
	}
	if p.OpenAICompatible != nil {
		s += "|" + p.OpenAICompatible.URL
	}
	return s
}

// secretStatus says whether the provider's secret is stored, as "set" or "not set".
func (p *Provider) secretStatus() (label, status string) {
	k, err := p.kind()
	if err != nil {
		return "", ""
	}
	set := false
	switch k {
	case kindOpenAICompatible:
		label, set = "API token", p.OpenAICompatible.APITokenSet
	default:
		label, set = "API key", p.keyConfig(k).APIKeySet
	}
	if set {
		return label, "set"
	}
	return label, "not set"
}

func secretLabel(kind string) (label, flag string) {
	switch kind {
	case kindOpenAICompatible:
		return "API token", "--api-key"
	case kindOpenAI:
		return "OpenAI API key", "--api-key"
	}
	return strings.ToUpper(kind[:1]) + kind[1:] + " API key", "--api-key"
}

func fromAPI(d gql.ModelProviderDetails) Provider {
	p := Provider{
		ID:          d.Id,
		Name:        d.Name,
		Description: cmdutil.Deref(d.Description),
		Tags:        cmdutil.TagsFromAPI(d.Tags),
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
	switch c := d.ProviderConfig.(type) {
	case *gql.ModelProviderDetailsProviderConfigModelProviderConfigAnthropic:
		p.Anthropic = &KeyConfig{APIKeySet: c.ApiKeySet}
	case *gql.ModelProviderDetailsProviderConfigModelProviderConfigOpenAI:
		p.OpenAI = &KeyConfig{APIKeySet: c.ApiKeySet}
	case *gql.ModelProviderDetailsProviderConfigModelProviderConfigCerebras:
		p.Cerebras = &KeyConfig{APIKeySet: c.ApiKeySet}
	case *gql.ModelProviderDetailsProviderConfigModelProviderConfigFireworks:
		p.Fireworks = &FireworksConfig{KeyConfig: KeyConfig{APIKeySet: c.ApiKeySet}, AccountID: cmdutil.Deref(c.AccountID)}
	case *gql.ModelProviderDetailsProviderConfigModelProviderConfigOpenAICompatible:
		p.OpenAICompatible = &OpenAICompatibleConfig{URL: c.Url, APITokenSet: c.ApiTokenSet}
	case *gql.ModelProviderDetailsProviderConfigModelProviderConfigGoogleGenAI:
		p.GoogleGenAI = &GoogleGenAIConfig{}
		if c.Gemini != nil {
			p.GoogleGenAI.Gemini = &KeyConfig{APIKeySet: c.Gemini.ApiKeySet}
		}
	}
	return p
}

// providerConfig builds the configuration to send. cur is the stored provider,
// or nil on create. It returns a nil config when no secret was given and no
// other setting changed, so that the server keeps the stored one.
func providerConfig(app *cmdutil.App, spec Provider, cur *Provider) (gql.ModelProviderType, *gql.ModelProviderConfigInput, error) {
	kind, err := spec.kind()
	if err != nil {
		return "", nil, err
	}
	typ := kindTypes[kind]
	if cur != nil {
		if ck, _ := cur.kind(); ck != kind {
			return "", nil, cmdutil.Errorf(cmdutil.ExitUsage, "Delete it and create a new one instead.", "provider %s is %s; its type cannot change to %s", cur.Name, ck, kind)
		}
	}
	label, flag := secretLabel(kind)
	var secret string
	switch kind {
	case kindOpenAICompatible:
		secret, err = readSecret(app, label, spec.OpenAICompatible.APIToken, spec.OpenAICompatible.APITokenEnv)
	default:
		k := spec.keyConfig(kind)
		secret, err = readSecret(app, label, k.APIKey, k.APIKeyEnv)
	}
	if err != nil {
		return "", nil, err
	}
	if secret == "" {
		if cur != nil && spec.settings() == cur.settings() {
			return typ, nil, nil
		}
		// An openai-compatible provider may have no token. Changing its URL
		// sends the whole configuration, so a stored token must be given again.
		needed := kind != kindOpenAICompatible || (cur != nil && cur.OpenAICompatible.APITokenSet)
		if needed {
			why := ""
			if cur != nil {
				why = fmt.Sprintf("changing the settings of provider %s sends its %s again, so it is required", cur.Name, label)
			}
			if secret, err = promptSecret(app, why, label, flag); err != nil {
				return "", nil, err
			}
		}
	}

	cfg := &gql.ModelProviderConfigInput{}
	switch kind {
	case kindAnthropic:
		cfg.Anthropic = &gql.ModelProviderConfigInputAnthropic{ApiKey: secret}
	case kindOpenAI:
		cfg.OpenAI = &gql.ModelProviderConfigInputOpenAI{ApiKey: secret}
	case kindCerebras:
		cfg.Cerebras = &gql.ModelProviderConfigInputCerebras{ApiKey: secret}
	case kindFireworks:
		cfg.Fireworks = &gql.ModelProviderConfigInputFireworks{ApiKey: secret}
		if id := spec.Fireworks.AccountID; id != "" {
			cfg.Fireworks.AccountID = gql.Opt(id)
		}
	case kindOpenAICompatible:
		if spec.OpenAICompatible.URL == "" {
			return "", nil, cmdutil.UsageError("an openai-compatible provider needs a url; pass --url")
		}
		cfg.OpenAICompatible = &gql.ModelProviderConfigInputOpenAICompatible{Url: spec.OpenAICompatible.URL}
		if secret != "" {
			cfg.OpenAICompatible.ApiToken = gql.Opt(secret)
		}
	case kindGemini:
		cfg.GoogleGenAI = &gql.ModelProviderConfigInputGoogleGenAI{Gemini: &gql.ModelProviderConfigInputGoogleGenAIGeminiApi{ApiKey: secret}}
	}
	return typ, cfg, nil
}

// Providers is the model provider resource. Other model commands use it to
// turn provider names into ids.
var Providers = &cmdutil.Resource[Provider]{
	WebList:  "models/providers",
	WebPage:  "models/providers/%s",
	WebLink:  "model-provider",
	Kind:     "model provider",
	Command:  "model-provider",
	IDPrefix: "cmpo",
	List: func(ctx context.Context, app *cmdutil.App, opts cmdutil.ListOptions) ([]Provider, error) {
		c, err := app.GQL()
		if err != nil {
			return nil, err
		}
		return cmdutil.ListAll(ctx, opts.Limit, func(ctx context.Context, first int, after *string) (cmdutil.Page[Provider], error) {
			resp, err := gql.ModelListProviders(ctx, c, first, after, cmdutil.OptString(opts.Query))
			if err != nil {
				return cmdutil.Page[Provider]{}, err
			}
			conn := resp.ModelProviders
			items := make([]Provider, len(conn.Edges))
			for i, e := range conn.Edges {
				items[i] = fromAPI(e.Node)
			}
			return cmdutil.Page[Provider]{Items: items, EndCursor: conn.PageInfo.EndCursor, HasNext: conn.PageInfo.HasNextPage}, nil
		})
	},
	Get: func(ctx context.Context, app *cmdutil.App, id string) (Provider, error) {
		c, err := app.GQL()
		if err != nil {
			return Provider{}, err
		}
		resp, err := gql.GetModelProvider(ctx, c, id)
		if err != nil {
			return Provider{}, err
		}
		return fromAPI(resp.ModelProvider), nil
	},
	Delete: func(ctx context.Context, app *cmdutil.App, ids []string) error {
		c, err := app.GQL()
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := gql.DeleteModelProvider(ctx, c, id); err != nil {
				return err
			}
		}
		return nil
	},
	ID:      func(p Provider) string { return p.ID },
	Name:    func(p Provider) string { return p.Name },
	Columns: []string{"id", "name", "type", "secret", "created"},
	Row: func(p Provider) []string {
		k, _ := p.kind()
		_, status := p.secretStatus()
		return []string{p.ID, p.Name, k, status, cmdutil.FormatTime(p.CreatedAt)}
	},
	Fields: func(p Provider) []output.Field {
		k, _ := p.kind()
		secretLabel, status := p.secretStatus()
		f := []output.Field{
			{Label: "ID", Value: p.ID},
			{Label: "Name", Value: p.Name},
			{Label: "Description", Value: p.Description},
			{Label: "Type", Value: k},
		}
		if p.OpenAICompatible != nil {
			f = append(f, output.Field{Label: "URL", Value: p.OpenAICompatible.URL})
		}
		if p.Fireworks != nil {
			f = append(f, output.Field{Label: "Account id", Value: p.Fireworks.AccountID})
		}
		return append(f,
			output.Field{Label: secretLabel, Value: status},
			output.Field{Label: "Tags", Value: cmdutil.FormatTags(p.Tags)},
			output.Field{Label: "Created", Value: p.CreatedAt},
			output.Field{Label: "Updated", Value: p.UpdatedAt},
		)
	},
}

func New(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "model-provider",
		Aliases: []string{"model-providers", "provider", "providers"},
		Short:   "Manage model providers: the credentials for a model service",
		Long: `A model provider holds the credentials for one model service, such as
Anthropic, OpenAI, Gemini, Fireworks, Cerebras or any service with an
OpenAI-compatible API. Model profiles and embedding profiles name a provider.

Types: ` + strings.Join(kinds, ", ") + `

` + secretHelp,
	}
	cmd.AddCommand(
		Providers.ListCmd(app),
		Providers.GetCmd(app),
		Providers.ViewCmd(app),
		newCreate(app),
		newUpdate(app),
		Providers.ApplyCmd(app, `  partite model-provider get my-anthropic -o yaml > provider.yaml
  partite model-provider apply -f provider.yaml

  # provider.yaml
  name: my-anthropic
  anthropic:
    api_key_env: ANTHROPIC_API_KEY

The key is sent only when the file gives one. Leave it out to keep the stored
key. `+secretHelp, func(ctx context.Context, spec Provider, id string) (Provider, error) {
			if id == "" {
				return create(ctx, app, spec)
			}
			cur, err := Providers.Get(ctx, app, id)
			if err != nil {
				return Provider{}, err
			}
			return update(ctx, app, id, spec, cur)
		}),
		Providers.DeleteCmd(app),
		newModels(app, false),
		newModels(app, true),
		newCapabilities(app),
	)
	return cmd
}

func create(ctx context.Context, app *cmdutil.App, spec Provider) (Provider, error) {
	typ, cfg, err := providerConfig(app, spec, nil)
	if err != nil {
		return Provider{}, err
	}
	c, err := app.GQL()
	if err != nil {
		return Provider{}, err
	}
	in := &gql.NewModelProvider{
		Name:           spec.Name,
		ProviderType:   typ,
		ProviderConfig: *cfg,
		Tags:           cmdutil.TagsToAPI(spec.Tags),
	}
	if spec.Description != "" {
		in.Description = gql.Opt(spec.Description)
	}
	if spec.Probe {
		in.Probe = cmdutil.Ptr(true)
	}
	resp, err := gql.CreateModelProvider(ctx, c, in)
	if err != nil {
		return Provider{}, err
	}
	return fromAPI(resp.CreateModelProvider), nil
}

// update makes the provider match spec. The configuration is sent only when
// spec gives a secret or changes another setting.
func update(ctx context.Context, app *cmdutil.App, id string, spec, cur Provider) (Provider, error) {
	_, cfg, err := providerConfig(app, spec, &cur)
	if err != nil {
		return Provider{}, err
	}
	c, err := app.GQL()
	if err != nil {
		return Provider{}, err
	}
	in := &gql.UpdateModelProvider{
		Name:           gql.Opt(spec.Name),
		Description:    gql.Opt(spec.Description),
		ProviderConfig: cfg,
		Tags:           gql.Opt(cmdutil.TagsToAPI(spec.Tags)),
	}
	if spec.Probe && cfg != nil {
		in.Probe = cmdutil.Ptr(true)
	}
	resp, err := gql.ModifyModelProvider(ctx, c, id, in)
	if err != nil {
		return Provider{}, err
	}
	return fromAPI(resp.UpdateModelProvider), nil
}

// configFlags are the flags that set a provider's type, settings and secret.
type configFlags struct {
	kind              string
	apiKey, apiKeyEnv string
	url, accountID    string
}

func (f *configFlags) add(cmd *cobra.Command, withType bool) {
	fl := cmd.Flags()
	if withType {
		fl.StringVar(&f.kind, "type", "", "provider type: "+strings.Join(kinds, ", "))
		_ = cmd.RegisterFlagCompletionFunc("type", cobra.FixedCompletions(kinds, cobra.ShellCompDirectiveNoFileComp))
	}
	fl.StringVar(&f.apiKey, "api-key", "", "API key, or token for openai-compatible: @- for standard input, @FILE for a file")
	fl.StringVar(&f.apiKeyEnv, "api-key-env", "", "read the API key or token from this environment variable")
	fl.StringVar(&f.url, "url", "", "base URL of the API (openai-compatible)")
	fl.StringVar(&f.accountID, "account-id", "", "Fireworks account id; the model list then includes the account's models (fireworks)")
	cmd.MarkFlagsMutuallyExclusive("api-key", "api-key-env")
}

// apply writes the flags that were passed into p.
func (f *configFlags) apply(app *cmdutil.App, cmd *cobra.Command, p *Provider) error {
	fl := cmd.Flags()
	if f.kind != "" {
		want := normalizeKind(f.kind)
		if _, ok := kindTypes[want]; !ok {
			return cmdutil.UsageError("unknown provider type %q; use one of %s", f.kind, strings.Join(kinds, ", "))
		}
		if have, err := p.kind(); err != nil {
			p.setKind(want)
		} else if have != want {
			return cmdutil.UsageError("--type %s does not match the spec file, which is %s", want, have)
		}
	}
	changed := false
	for _, name := range []string{"api-key", "api-key-env", "url", "account-id"} {
		changed = changed || fl.Changed(name)
	}
	if !changed {
		return nil
	}
	kind, err := p.kind()
	if err != nil {
		return err
	}
	only := func(flag string, ok bool) error {
		if fl.Changed(flag) && !ok {
			return cmdutil.UsageError("--%s does not apply to %s providers", flag, kind)
		}
		return nil
	}
	for flag, ok := range map[string]bool{
		"url":        kind == kindOpenAICompatible,
		"account-id": kind == kindFireworks,
	} {
		if err := only(flag, ok); err != nil {
			return err
		}
	}
	warnLiteral(app, "--api-key", f.apiKey)
	switch kind {
	case kindOpenAICompatible:
		o := p.OpenAICompatible
		if fl.Changed("url") {
			o.URL = f.url
		}
		if fl.Changed("api-key") || fl.Changed("api-key-env") {
			o.APIToken, o.APITokenEnv = f.apiKey, f.apiKeyEnv
		}
	default:
		if fl.Changed("api-key") || fl.Changed("api-key-env") {
			k := p.keyConfig(kind)
			k.APIKey, k.APIKeyEnv = f.apiKey, f.apiKeyEnv
		}
		if fl.Changed("account-id") {
			p.Fireworks.AccountID = f.accountID
		}
	}
	return nil
}

func newCreate(app *cmdutil.App) *cobra.Command {
	var spec Provider
	var file string
	var tags []string
	var flags configFlags
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a model provider",
		Long: `Create a model provider. Pass --type and the provider's key, or read the
whole provider from a file with -f. Flags override the file.

Types: ` + strings.Join(kinds, ", ") + `

With --probe, the server lists the provider's models with the key first, and
refuses a key the provider rejects.

` + secretHelp,
		Example: `  partite model-provider create my-anthropic --type anthropic --api-key-env ANTHROPIC_API_KEY
  op read op://ai/openai/key | partite model-provider create my-openai --type openai --api-key @-
  partite model-provider create local-llm --type openai-compatible --url http://localhost:11434/v1
  partite model-provider create my-gemini --type gemini --probe   # asks for the key`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file != "" {
				if err := app.ReadSpec(file, &spec); err != nil {
					return err
				}
			}
			if len(args) == 1 {
				spec.Name = args[0]
			}
			if spec.Name == "" {
				return cmdutil.UsageError("a name is required")
			}
			if cmd.Flags().Changed("tag") {
				t, err := cmdutil.ParseTags(tags)
				if err != nil {
					return err
				}
				spec.Tags = t
			}
			if err := flags.apply(app, cmd, &spec); err != nil {
				return err
			}
			p, err := create(cmd.Context(), app, spec)
			if err != nil {
				return err
			}
			app.Printer().Done("Created model provider %s (%s).", p.Name, p.ID)
			if app.Format() == output.Table {
				return nil
			}
			return Providers.Show(app, p)
		},
	}
	flags.add(cmd, true)
	cmd.Flags().StringVar(&spec.Description, "description", "", "description")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "tag as name=value; repeat for more")
	cmd.Flags().BoolVar(&spec.Probe, "probe", false, "check the key with the provider before saving")
	cmd.Flags().StringVarP(&file, "file", "f", "", "read the provider from a YAML or JSON file")
	return cmd
}

func newUpdate(app *cmdutil.App) *cobra.Command {
	var name, description string
	var tags []string
	var probe bool
	var flags configFlags
	cmd := &cobra.Command{
		Use:   "update <name-or-id>",
		Short: "Change a model provider's name, settings or key",
		Long: `Change a model provider. Only the flags you pass change. A provider's type
cannot change.

Changing a setting such as --url or --location sends the whole configuration,
so the key is needed again; at a terminal you are asked for it.

` + secretHelp,
		Example: `  partite model-provider update my-anthropic --api-key-env NEW_ANTHROPIC_KEY --probe
  partite model-provider update local-llm --url http://gpu-box:8000/v1
  partite model-provider update my-openai --description "Team key" --tag team=ml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := Providers.Resolve(cmd.Context(), app, args[0])
			if err != nil {
				return err
			}
			cur, err := Providers.Get(cmd.Context(), app, id)
			if err != nil {
				return err
			}
			// Copy cur through JSON so that changes to spec's blocks leave cur alone.
			var spec Provider
			raw, _ := json.Marshal(cur)
			_ = json.Unmarshal(raw, &spec)
			f := cmd.Flags()
			if f.Changed("name") {
				spec.Name = name
			}
			if f.Changed("description") {
				spec.Description = description
			}
			if f.Changed("tag") {
				if spec.Tags, err = cmdutil.ParseTags(tags); err != nil {
					return err
				}
			}
			spec.Probe = probe
			if err := flags.apply(app, cmd, &spec); err != nil {
				return err
			}
			changed := false
			for _, name := range []string{"name", "description", "tag", "api-key", "api-key-env", "url", "account-id"} {
				changed = changed || f.Changed(name)
			}
			if !changed {
				return cmdutil.UsageError("nothing to change; see --help for the flags")
			}
			p, err := update(cmd.Context(), app, id, spec, cur)
			if err != nil {
				return err
			}
			app.Printer().Done("Updated model provider %s.", p.Name)
			if app.Format() == output.Table {
				return nil
			}
			return Providers.Show(app, p)
		},
	}
	flags.add(cmd, false)
	cmd.Flags().StringVar(&name, "name", "", "new name")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "tags as name=value, replacing the current ones; repeat for more")
	cmd.Flags().BoolVar(&probe, "probe", false, "check a new key with the provider before saving")
	return cmd
}
