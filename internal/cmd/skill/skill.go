// Package skill manages skills: instructions and files an agent loads when a
// task calls for them.
package skill

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/partite-ai/partite-cli/internal/cmdutil"
	gql "github.com/partite-ai/partite-cli/internal/graphql"
	"github.com/partite-ai/partite-cli/internal/output"
	"github.com/spf13/cobra"
)

// Skill is the view printed by get and read by -f. Files maps each path in the
// skill folder to its text. declared_name, description and instructions come
// from SKILL.md and are ignored on input.
type Skill struct {
	ID           string            `json:"id,omitempty"`
	Name         string            `json:"name"`
	DeclaredName string            `json:"declared_name,omitempty"`
	Description  string            `json:"description,omitempty"`
	Instructions string            `json:"instructions,omitempty"`
	Files        map[string]string `json:"files,omitempty"`
	AgentCount   int               `json:"agent_count,omitempty"`
	CreatedAt    string            `json:"created_at,omitempty"`
	UpdatedAt    string            `json:"updated_at,omitempty"`
}

// fromAPI reads a skill the server returned. A file too large for its content
// to be returned is read from its download URL.
func fromAPI(ctx context.Context, d gql.SkillDetails) (Skill, error) {
	s := Skill{
		ID:           d.Id,
		Name:         d.Name,
		DeclaredName: d.DeclaredName,
		Description:  d.Description,
		Instructions: d.Instructions,
		Files:        make(map[string]string, len(d.Files)),
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
	for _, f := range d.Files {
		switch {
		case f.Content != nil:
			s.Files[f.Path] = *f.Content
		case f.DownloadURL != nil:
			content, err := gql.Download(ctx, cmdutil.PlainHTTP(), *f.DownloadURL)
			if err != nil {
				return Skill{}, fmt.Errorf("read %s: %w", f.Path, err)
			}
			s.Files[f.Path] = string(content)
		default:
			return Skill{}, fmt.Errorf("the server returned no content for %s", f.Path)
		}
	}
	return s, nil
}

// Resource is exported so that agents can refer to skills by name.
var Resource = &cmdutil.Resource[Skill]{
	WebList:  "tools/skills",
	WebPage:  "tools/skills/%s",
	WebLink:  "skill",
	Kind:     "skill",
	Command:  "skill",
	IDPrefix: "cskl",
	List: func(ctx context.Context, app *cmdutil.App, opts cmdutil.ListOptions) ([]Skill, error) {
		c, err := app.GQL()
		if err != nil {
			return nil, err
		}
		return cmdutil.ListAll(ctx, opts.Limit, func(ctx context.Context, first int, after *string) (cmdutil.Page[Skill], error) {
			resp, err := gql.AgentListSkills(ctx, c, first, after, cmdutil.OptString(opts.Query))
			if err != nil {
				return cmdutil.Page[Skill]{}, err
			}
			conn := resp.Skills
			items := make([]Skill, len(conn.Edges))
			for i, e := range conn.Edges {
				n := e.Node
				items[i] = Skill{ID: n.Id, Name: n.Name, DeclaredName: n.DeclaredName, Description: n.Description, AgentCount: n.AgentUsageCount, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt}
			}
			return cmdutil.Page[Skill]{Items: items, EndCursor: conn.PageInfo.EndCursor, HasNext: conn.PageInfo.HasNextPage}, nil
		})
	},
	Get: func(ctx context.Context, app *cmdutil.App, id string) (Skill, error) {
		c, err := app.GQL()
		if err != nil {
			return Skill{}, err
		}
		resp, err := gql.GetSkill(ctx, c, id)
		if err != nil {
			return Skill{}, err
		}
		if resp.Skill == nil {
			return Skill{}, cmdutil.NotFound("skill", id, "partite skill list")
		}
		return fromAPI(ctx, *resp.Skill)
	},
	Delete: func(ctx context.Context, app *cmdutil.App, ids []string) error {
		c, err := app.GQL()
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := gql.DeleteSkill(ctx, c, id); err != nil {
				return err
			}
		}
		return nil
	},
	ID:      func(s Skill) string { return s.ID },
	Name:    func(s Skill) string { return s.Name },
	Columns: []string{"id", "name", "declared name", "agents", "description"},
	Row: func(s Skill) []string {
		return []string{s.ID, s.Name, s.DeclaredName, strconv.Itoa(s.AgentCount), s.Description}
	},
	Fields: func(s Skill) []output.Field {
		return []output.Field{
			{Label: "ID", Value: s.ID},
			{Label: "Name", Value: s.Name},
			{Label: "Declared name", Value: s.DeclaredName},
			{Label: "Description", Value: s.Description},
			{Label: "Files", Value: strings.Join(sortedPaths(s.Files), "\n")},
			{Label: "Created", Value: s.CreatedAt},
			{Label: "Updated", Value: s.UpdatedAt},
		}
	},
}

func sortedPaths(files map[string]string) []string {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// toSource uploads each file, and returns the files as the source of a skill.
func toSource(ctx context.Context, app *cmdutil.App, files map[string]string) (gql.SkillSource, error) {
	if _, ok := files["SKILL.md"]; !ok {
		return gql.SkillSource{}, cmdutil.UsageError("a skill needs a SKILL.md file")
	}
	src := gql.SkillSource{Files: make([]gql.SkillSourceFile, 0, len(files))}
	for _, p := range sortedPaths(files) {
		f, err := uploadFile(ctx, app, p, files[p])
		if err != nil {
			return gql.SkillSource{}, err
		}
		src.Files = append(src.Files, f)
	}
	return src, nil
}

// uploadFile uploads the content of the file at path in a skill folder.
func uploadFile(ctx context.Context, app *cmdutil.App, path, content string) (gql.SkillSourceFile, error) {
	c, err := app.GQL()
	if err != nil {
		return gql.SkillSourceFile{}, err
	}
	upload, err := gql.Upload(ctx, c, cmdutil.PlainHTTP(), strings.NewReader(content), int64(len(content)), nil)
	if err != nil {
		return gql.SkillSourceFile{}, fmt.Errorf("%s: %w", path, err)
	}
	return gql.SkillSourceFile{Path: path, ContentUpload: upload}, nil
}

// readDir reads every text file under dir, by its slash-separated path within
// dir. Hidden files and folders are skipped, and so are binary files, with a
// warning.
func readDir(app *cmdutil.App, dir string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != dir && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			fmt.Fprintf(app.Err, "Skipping %s: not a text file.\n", rel)
			return nil
		}
		files[rel] = string(data)
		return nil
	})
	if err != nil {
		return nil, cmdutil.UsageError("read %s: %s", dir, err)
	}
	if _, ok := files["SKILL.md"]; !ok {
		return nil, cmdutil.UsageError("%s has no SKILL.md", dir)
	}
	return files, nil
}

func New(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "skill",
		Aliases: []string{"skills"},
		Short:   "Manage skills: instructions and files an agent loads when it needs them",
		Long: `A skill is a folder with a SKILL.md file and any files it refers to. SKILL.md
starts with front matter that declares the skill's name and a description that
tells the agent when to use it. An agent's prompt lists its skills, and the
agent loads the rest when a task calls for it.

Create a skill from a folder with partite skill create NAME --dir FOLDER, and
give it to an agent with partite agent update AGENT --skill NAME.`,
	}
	cmd.AddCommand(
		Resource.ListCmd(app),
		Resource.GetCmd(app),
		Resource.ViewCmd(app),
		newCreate(app),
		newUpdate(app),
		Resource.ApplyCmd(app, "  partite skill apply -f pdf-forms.yaml", func(ctx context.Context, spec Skill, id string) (Skill, error) {
			src, err := toSource(ctx, app, spec.Files)
			if err != nil {
				return Skill{}, err
			}
			if id == "" {
				return create(ctx, app, spec.Name, src)
			}
			return update(ctx, app, id, &gql.UpdateSkill{Name: gql.Opt(spec.Name), Source: gql.Opt(src)})
		}),
		Resource.DeleteCmd(app),
		newCopy(app),
		newFiles(app),
		newPull(app),
	)
	return cmd
}

func create(ctx context.Context, app *cmdutil.App, name string, src gql.SkillSource) (Skill, error) {
	c, err := app.GQL()
	if err != nil {
		return Skill{}, err
	}
	resp, err := gql.CreateSkill(ctx, c, &gql.NewSkill{Name: name, Source: src})
	if err != nil {
		return Skill{}, err
	}
	return fromAPI(ctx, resp.CreateSkill)
}

func update(ctx context.Context, app *cmdutil.App, id string, in *gql.UpdateSkill) (Skill, error) {
	c, err := app.GQL()
	if err != nil {
		return Skill{}, err
	}
	resp, err := gql.ModifySkill(ctx, c, id, in)
	if err != nil {
		return Skill{}, err
	}
	return fromAPI(ctx, resp.UpdateSkill)
}

func newCreate(app *cmdutil.App) *cobra.Command {
	var dir, file string
	cmd := &cobra.Command{
		Use:   "create <name> --dir <folder>",
		Short: "Create a skill from a folder",
		Long: `Create a skill from a folder that has a SKILL.md file. Every text file in the
folder is uploaded; hidden files and binary files are skipped.`,
		Example: `  partite skill create pdf-forms --dir ./skills/pdf-forms
  partite skill create -f pdf-forms.yaml`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var spec Skill
			if file != "" && dir != "" {
				return cmdutil.UsageError("--dir and --file cannot be used together")
			}
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
			if dir != "" {
				files, err := readDir(app, dir)
				if err != nil {
					return err
				}
				spec.Files = files
			}
			if len(spec.Files) == 0 {
				return cmdutil.UsageError("pass --dir with the skill's folder")
			}
			src, err := toSource(cmd.Context(), app, spec.Files)
			if err != nil {
				return err
			}
			s, err := create(cmd.Context(), app, spec.Name, src)
			if err != nil {
				return err
			}
			app.Printer().Done("Created skill %s (%s) with %s.", s.Name, s.ID, cmdutil.Plural(len(s.Files), "file"))
			if app.Format() == output.Table {
				return nil
			}
			return Resource.Show(app, s)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "folder with SKILL.md and the skill's other files")
	cmd.Flags().StringVarP(&file, "file", "f", "", "read the skill from a YAML or JSON file")
	return cmd
}

func newUpdate(app *cmdutil.App) *cobra.Command {
	var name, dir string
	var put, remove []string
	cmd := &cobra.Command{
		Use:   "update <name-or-id>",
		Short: "Rename a skill or change its files",
		Long: `Rename a skill or change its files. --dir replaces all of the skill's files
with the folder's. --put and --remove change some files and keep the rest.
Every agent that uses the skill gets the new files.`,
		Example: `  partite skill update pdf-forms --dir ./skills/pdf-forms
  partite skill update pdf-forms --put reference/forms.md=./forms.md --remove old.md
  partite skill update pdf-forms --name pdf-forms-v2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			editing := len(put) > 0 || len(remove) > 0
			if dir != "" && editing {
				return cmdutil.UsageError("--dir replaces every file, so it cannot be used with --put or --remove")
			}
			if !f.Changed("name") && dir == "" && !editing {
				return cmdutil.UsageError("nothing to change; pass --name, --dir, --put or --remove")
			}
			id, err := Resource.Resolve(cmd.Context(), app, args[0])
			if err != nil {
				return err
			}
			in := &gql.UpdateSkill{}
			if f.Changed("name") {
				in.Name = gql.Opt(name)
			}
			if dir != "" {
				files, err := readDir(app, dir)
				if err != nil {
					return err
				}
				src, err := toSource(cmd.Context(), app, files)
				if err != nil {
					return err
				}
				in.Source = gql.Opt(src)
			}
			var s Skill
			if in.Name.Set || in.Source.Set {
				if s, err = update(cmd.Context(), app, id, in); err != nil {
					return err
				}
			}
			if editing {
				edit := &gql.EditSkillFiles{Remove: remove}
				for _, p := range put {
					skillPath, local, ok := strings.Cut(p, "=")
					if !ok {
						local = p
					}
					data, err := os.ReadFile(local)
					if err != nil {
						return cmdutil.UsageError("read %s: %s", local, err)
					}
					if !utf8.Valid(data) {
						return cmdutil.UsageError("%s is not a text file", local)
					}
					file, err := uploadFile(cmd.Context(), app, filepath.ToSlash(skillPath), string(data))
					if err != nil {
						return err
					}
					edit.Put = append(edit.Put, file)
				}
				c, err := app.GQL()
				if err != nil {
					return err
				}
				resp, err := gql.AgentEditSkillFiles(cmd.Context(), c, id, edit)
				if err != nil {
					return err
				}
				if s, err = fromAPI(cmd.Context(), resp.EditSkillFiles); err != nil {
					return err
				}
			}
			app.Printer().Done("Updated skill %s.", s.Name)
			if app.Format() == output.Table {
				return nil
			}
			return Resource.Show(app, s)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new name")
	cmd.Flags().StringVar(&dir, "dir", "", "folder whose files replace all of the skill's files")
	cmd.Flags().StringArrayVar(&put, "put", nil, "add or replace a file, as PATH=LOCAL_FILE, or LOCAL_FILE to use the same path; repeat for more")
	cmd.Flags().StringArrayVar(&remove, "remove", nil, "path of a file to remove; repeat for more")
	return cmd
}

func newCopy(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "copy <name-or-id> <new-name>",
		Short: "Copy a skill and its files to a new name",
		Long: `Copy a skill and its files to a new name. Agents that use the skill keep using
it; none use the copy until you add it to them.`,
		Example: "  partite skill copy pdf-forms pdf-forms-test",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := Resource.Resolve(cmd.Context(), app, args[0])
			if err != nil {
				return err
			}
			c, err := app.GQL()
			if err != nil {
				return err
			}
			resp, err := gql.AgentCopySkill(cmd.Context(), c, id, args[1])
			if err != nil {
				return err
			}
			s, err := fromAPI(cmd.Context(), resp.CopySkill)
			if err != nil {
				return err
			}
			app.Printer().Done("Copied skill %s to %s (%s).", args[0], s.Name, s.ID)
			if app.Format() == output.Table {
				return nil
			}
			return Resource.Show(app, s)
		},
	}
}

type fileInfo struct {
	Path string `json:"path"`
	Size int    `json:"size"`
}

func newFiles(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:     "files <name-or-id>",
		Short:   "List a skill's files",
		Example: "  partite skill files pdf-forms",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := Resource.Resolve(cmd.Context(), app, args[0])
			if err != nil {
				return err
			}
			c, err := app.GQL()
			if err != nil {
				return err
			}
			resp, err := gql.AgentSkillFiles(cmd.Context(), c, id)
			if err != nil {
				return err
			}
			if resp.Skill == nil {
				return cmdutil.NotFound("skill", args[0], "partite skill list")
			}
			files := make([]fileInfo, len(resp.Skill.Files))
			rows := make([][]string, len(files))
			for i, f := range resp.Skill.Files {
				files[i] = fileInfo{Path: f.Path, Size: f.Size}
				rows[i] = []string{f.Path, strconv.Itoa(f.Size)}
			}
			return app.Printer().List(files, []string{"path", "size"}, rows)
		},
	}
}

func newPull(app *cmdutil.App) *cobra.Command {
	var overwrite bool
	cmd := &cobra.Command{
		Use:   "pull <name-or-id> <folder>",
		Short: "Write a skill's files to a folder",
		Long: `Write a skill's files to a folder, creating it if needed. Existing files are
not replaced unless you pass --overwrite. Edit the files and upload them again
with partite skill update NAME --dir FOLDER.`,
		Example: "  partite skill pull pdf-forms ./skills/pdf-forms",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := Resource.Resolve(cmd.Context(), app, args[0])
			if err != nil {
				return err
			}
			s, err := Resource.Get(cmd.Context(), app, id)
			if err != nil {
				return err
			}
			dir := args[1]
			paths := sortedPaths(s.Files)
			targets := make([]string, len(paths))
			for i, p := range paths {
				clean := path.Clean(p)
				if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
					return cmdutil.Errorf(cmdutil.ExitError, "", "the skill has a file outside its folder: %s", p)
				}
				targets[i] = filepath.Join(dir, filepath.FromSlash(clean))
				if !overwrite {
					if _, err := os.Stat(targets[i]); err == nil {
						return &cmdutil.Error{Code: cmdutil.ExitConflict, Msg: fmt.Sprintf("%s already exists", targets[i]), Hint: "Pass --overwrite to replace it."}
					}
				}
			}
			for i, p := range paths {
				if err := os.MkdirAll(filepath.Dir(targets[i]), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(targets[i], []byte(s.Files[p]), 0o644); err != nil {
					return err
				}
			}
			app.Printer().Done("Wrote %s to %s.", cmdutil.Plural(len(paths), "file"), dir)
			if app.Format() == output.Table {
				return nil
			}
			return app.Printer().Data(map[string]any{"dir": dir, "files": paths})
		},
	}
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "replace files that already exist")
	return cmd
}
