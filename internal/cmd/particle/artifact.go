package particle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/partite-ai/partite-cli/internal/cmdutil"
	gql "github.com/partite-ai/partite-cli/internal/graphql"
	"github.com/partite-ai/partite-cli/internal/output"
	"github.com/spf13/cobra"
)

// Artifact is an uploaded .particle file, as the server parsed its manifest.
type Artifact struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Version      string               `json:"version"`
	Description  string               `json:"description,omitempty"`
	SourceHash   string               `json:"source_hash"`
	Capabilities ArtifactCapabilities `json:"capabilities"`
	Credentials  []ArtifactCredential `json:"credentials,omitempty"`
	Tools        []Tool               `json:"tools,omitempty"`
	CreatedAt    string               `json:"created_at,omitempty"`
}

type ArtifactCapabilities struct {
	HTTP *struct {
		AllowedHosts []string `json:"allowed_hosts"`
	} `json:"http,omitempty"`
}

// ArtifactCredential is a credential the particle needs. The install picks one
// of its methods.
type ArtifactCredential struct {
	Name     string             `json:"name"`
	Hosts    []string           `json:"hosts,omitempty"`
	Required bool               `json:"required"`
	Methods  []CredentialMethod `json:"methods"`
}

type CredentialMethod struct {
	Name           string          `json:"name"`
	Type           string          `json:"type"`
	Description    string          `json:"description,omitempty"`
	APIKeyLocation *APIKeyLocation `json:"apikey_location,omitempty"`
	OAuth2         *OAuth2Details  `json:"oauth2,omitempty"`
	SigningKey     *struct {
		Algorithm string `json:"algorithm"`
	} `json:"signing_key,omitempty"`
}

type APIKeyLocation struct {
	Kind   string `json:"kind"`
	Name   string `json:"name,omitempty"`
	Scheme string `json:"scheme,omitempty"`
}

type OAuth2Details struct {
	AuthorizationURL string   `json:"authorization_url,omitempty"`
	TokenURL         string   `json:"token_url,omitempty"`
	DeviceAuthURL    string   `json:"device_auth_url,omitempty"`
	Scopes           []string `json:"scopes,omitempty"`
	Flows            []string `json:"flows,omitempty"`
}

// Tool is a tool a particle declares.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

func artifactFromAPI(d gql.ParticleArtifactDetails) Artifact {
	a := Artifact{
		ID:          d.Id,
		Name:        d.Name,
		Version:     d.Version,
		Description: cmdutil.Deref(d.Description),
		SourceHash:  d.Sha256,
		CreatedAt:   d.CreatedAt,
	}
	if h := d.Capabilities.Http; h != nil {
		a.Capabilities.HTTP = &struct {
			AllowedHosts []string `json:"allowed_hosts"`
		}{h.AllowedHosts}
	}
	for _, c := range d.Credentials {
		cred := ArtifactCredential{Name: c.Name, Hosts: c.Hosts, Required: c.Required}
		for _, m := range c.Methods {
			method := CredentialMethod{Name: m.Name, Type: string(m.Type), Description: cmdutil.Deref(m.Description)}
			if l := m.ApiKeyLocation; l != nil {
				method.APIKeyLocation = &APIKeyLocation{Kind: string(l.Kind), Name: cmdutil.Deref(l.Name), Scheme: cmdutil.Deref(l.Scheme)}
			}
			if o := m.Oauth2; o != nil {
				details := &OAuth2Details{
					AuthorizationURL: cmdutil.Deref(o.AuthorizationUrl),
					TokenURL:         cmdutil.Deref(o.TokenUrl),
					DeviceAuthURL:    cmdutil.Deref(o.DeviceAuthUrl),
					Scopes:           o.Scopes,
				}
				for _, f := range o.Flows {
					details.Flows = append(details.Flows, string(f))
				}
				method.OAuth2 = details
			}
			if s := m.SigningKey; s != nil {
				method.SigningKey = &struct {
					Algorithm string `json:"algorithm"`
				}{s.Algorithm}
			}
			cred.Methods = append(cred.Methods, method)
		}
		a.Credentials = append(a.Credentials, cred)
	}
	for _, t := range d.Tools {
		a.Tools = append(a.Tools, Tool{Name: t.Name, Description: cmdutil.Deref(t.Description), InputSchema: t.InputSchema})
	}
	return a
}

func showArtifact(app *cmdutil.App, a Artifact) error {
	var creds []string
	for _, c := range a.Credentials {
		methods := make([]string, len(c.Methods))
		for i, m := range c.Methods {
			methods[i] = fmt.Sprintf("%s (%s)", m.Name, m.Type)
		}
		req := "optional"
		if c.Required {
			req = "required"
		}
		creds = append(creds, fmt.Sprintf("%s, %s: %s", c.Name, req, strings.Join(methods, ", ")))
	}
	tools := make([]string, len(a.Tools))
	for i, t := range a.Tools {
		tools[i] = t.Name
	}
	hosts := ""
	if a.Capabilities.HTTP != nil {
		hosts = strings.Join(a.Capabilities.HTTP.AllowedHosts, ", ")
	}
	return app.Printer().Object(a, a.ID, []output.Field{
		{Label: "ID", Value: a.ID},
		{Label: "Name", Value: a.Name},
		{Label: "Version", Value: a.Version},
		{Label: "Description", Value: a.Description},
		{Label: "SHA-256", Value: a.SourceHash},
		{Label: "HTTP hosts", Value: hosts},
		{Label: "Credentials", Value: strings.Join(creds, "\n")},
		{Label: "Tools", Value: strings.Join(tools, ", ")},
		{Label: "Created", Value: a.CreatedAt},
	})
}

func getArtifact(ctx context.Context, app *cmdutil.App, id string) (Artifact, error) {
	c, err := app.GQL()
	if err != nil {
		return Artifact{}, err
	}
	resp, err := gql.GetParticleArtifact(ctx, c, id)
	if err != nil {
		return Artifact{}, err
	}
	if resp.ParticleArtifact == nil {
		return Artifact{}, &cmdutil.Error{Code: cmdutil.ExitNotFound, Msg: fmt.Sprintf("particle artifact %q not found", id), Hint: "Upload one with: partite particle upload <file.particle>"}
	}
	return artifactFromAPI(*resp.ParticleArtifact), nil
}

func upload(ctx context.Context, app *cmdutil.App, path string) (Artifact, error) {
	f, err := os.Open(path)
	if err != nil {
		return Artifact{}, cmdutil.UsageError("open %s: %s", path, err)
	}
	defer f.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, f)
	if err != nil {
		return Artifact{}, cmdutil.UsageError("read %s: %s", path, err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return Artifact{}, err
	}
	c, err := app.GQL()
	if err != nil {
		return Artifact{}, err
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	uploadID, err := gql.Upload(ctx, c, http.DefaultClient, f, size, &digest)
	if err != nil {
		return Artifact{}, err
	}
	resp, err := gql.UploadParticleArtifact(ctx, c, uploadID)
	if err != nil {
		return Artifact{}, err
	}
	return artifactFromAPI(resp.UploadParticleArtifact), nil
}

// artifactID returns ref when it is an artifact id, and otherwise uploads the
// file at ref and returns the new artifact's id.
func artifactID(ctx context.Context, app *cmdutil.App, ref string) (string, error) {
	if cmdutil.IsID(ref, "cpar") {
		return ref, nil
	}
	if _, err := os.Stat(ref); err != nil {
		return "", cmdutil.UsageError("%s is neither a particle artifact id (cpar_...) nor a file", ref)
	}
	a, err := upload(ctx, app, ref)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(app.Err, "Uploaded %s as artifact %s (%s %s).\n", ref, a.ID, a.Name, a.Version)
	return a.ID, nil
}

func newUpload(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "upload <file.particle>",
		Short: "Upload a .particle file and show what it declares",
		Long: `Upload a .particle file. This makes an artifact: the particle's code and
manifest. It does nothing until it is installed with partite particle install
or set on an installed particle with partite particle update --artifact.`,
		Example: `  partite particle upload weather.particle
  partite particle upload weather.particle -o id`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := upload(cmd.Context(), app, args[0])
			if err != nil {
				return err
			}
			return showArtifact(app, a)
		},
	}
}

func newArtifact(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "artifact",
		Aliases: []string{"artifacts"},
		Short:   "Show uploaded particle artifacts",
	}
	cmd.AddCommand(&cobra.Command{
		Use:     "get <artifact-id>",
		Aliases: []string{"show"},
		Short:   "Show a particle artifact: its credentials, tools and capabilities",
		Example: "  partite particle artifact get cpar_7Hq2...",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := getArtifact(cmd.Context(), app, args[0])
			if err != nil {
				return err
			}
			return showArtifact(app, a)
		},
	})
	return cmd
}
