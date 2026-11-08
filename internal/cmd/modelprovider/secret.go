package modelprovider

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/partite-ai/partite-cli/internal/cmdutil"
	"golang.org/x/term"
)

// secretHelp explains secret input. It is shared by the commands that take a key.
const secretHelp = `Keys and tokens are never printed. get shows only whether one is set.
Give a key in one of these ways, safest first:

  --api-key-env NAME    read it from the environment variable NAME
  --api-key @-          read it from standard input
  --api-key @FILE       read it from a file
  (nothing)             at a terminal, you are asked for it; typing is hidden
  --api-key VALUE       works, but the key can end up in your shell history

In a spec file, set api_key_env: NAME, or api_key: ${NAME} to read an
environment variable. The same goes for api_token (openai_compatible).`

var envRef = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// readSecret returns the secret from value or envName, or "" when both are
// empty. value may be @-, @FILE, or text with ${NAME} references.
func readSecret(app *cmdutil.App, label, value, envName string) (string, error) {
	if value != "" && envName != "" {
		return "", cmdutil.UsageError("give the %s either directly or from an environment variable, not both", label)
	}
	if envName != "" {
		s := os.Getenv(envName)
		if s == "" {
			return "", cmdutil.UsageError("the environment variable %s, which should hold the %s, is empty or not set", envName, label)
		}
		return s, nil
	}
	if strings.HasPrefix(value, "@") {
		s, err := app.ReadValue(value)
		if err != nil {
			return "", err
		}
		s = strings.TrimRight(s, "\r\n")
		if s == "" {
			return "", cmdutil.UsageError("%s is empty", value)
		}
		return s, nil
	}
	var missing string
	s := envRef.ReplaceAllStringFunc(value, func(ref string) string {
		name := envRef.FindStringSubmatch(ref)[1]
		v := os.Getenv(name)
		if v == "" && missing == "" {
			missing = name
		}
		return v
	})
	if missing != "" {
		return "", cmdutil.UsageError("the environment variable %s, which should hold the %s, is empty or not set", missing, label)
	}
	return s, nil
}

// promptSecret asks for a secret at a terminal without echoing it. Without a
// terminal it fails with how to pass the secret instead. why, when set, says
// why the secret is needed.
func promptSecret(app *cmdutil.App, why, label, flag string) (string, error) {
	in, ok := app.In.(*os.File)
	if !app.Interactive() || !ok {
		if why == "" {
			why = fmt.Sprintf("the %s is required", label)
		}
		return "", &cmdutil.Error{
			Code: cmdutil.ExitUsage,
			Msg:  why,
			Hint: fmt.Sprintf("Pass %s-env NAME or %s @-, or set it in the spec file. See --help.", flag, flag),
		}
	}
	if why != "" {
		fmt.Fprintf(app.Err, "%s.\n", strings.ToUpper(why[:1])+why[1:])
	}
	fmt.Fprintf(app.Err, "%s: ", label)
	b, err := term.ReadPassword(int(in.Fd()))
	fmt.Fprintln(app.Err)
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", cmdutil.UsageError("no %s given", label)
	}
	return s, nil
}

// warnLiteral warns when a secret flag holds the secret itself.
func warnLiteral(app *cmdutil.App, flag, value string) {
	if value != "" && !strings.HasPrefix(value, "@") && !envRef.MatchString(value) {
		fmt.Fprintf(app.Err, "Warning: a secret given as %s VALUE can end up in your shell history. Prefer %s-env NAME or %s @-.\n", flag, flag, flag)
	}
}
