package exedev

import "strings"

type commandBuilder struct {
	args []string
}

func newCommand(args ...string) commandBuilder {
	return commandBuilder{args: append([]string(nil), args...)}
}

func (b commandBuilder) append(args ...string) commandBuilder {
	b.args = append(b.args, args...)
	return b
}

func (b commandBuilder) render() string {
	return joinArgs(b.args)
}

func joinArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = quoteArg(arg)
	}
	return strings.Join(quoted, " ")
}

func sanitizeCommandForLog(command string) string {
	args, ok := splitCommandArgs(command)
	if !ok {
		return command
	}
	return joinArgs(redactCommandArgs(args))
}

func redactCommandArgs(args []string) []string {
	redacted := append([]string(nil), args...)
	for i := 0; i < len(redacted); i++ {
		arg := redacted[i]
		if arg == "--env" && i+1 < len(redacted) {
			redacted[i+1] = sanitizeEnvArg(redacted[i+1])
			i++
			continue
		}
		if envArg, ok := strings.CutPrefix(arg, "--env="); ok {
			redacted[i] = "--env=" + sanitizeEnvArg(envArg)
		}
	}
	return redacted
}

func sanitizeEnvArg(arg string) string {
	key, _, ok := strings.Cut(arg, "=")
	if !ok || !shouldRedactEnvValue(key) {
		return arg
	}
	return key + "=<redacted>"
}

func shouldRedactEnvValue(key string) bool {
	key = strings.ToUpper(key)
	for _, marker := range []string{"SECRET", "TOKEN", "PASSWORD", "PASS", "KEY", "CREDENTIAL", "AUTH"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}

func splitCommandArgs(command string) ([]string, bool) {
	var args []string
	var current strings.Builder
	inSingleQuote := false
	for i := 0; i < len(command); i++ {
		ch := command[i]
		switch {
		case inSingleQuote:
			if ch == '\'' {
				if i+3 < len(command) && command[i+1] == '\\' && command[i+2] == '\'' && command[i+3] == '\'' {
					current.WriteByte('\'')
					i += 3
					continue
				}
				inSingleQuote = false
				continue
			}
			current.WriteByte(ch)
		case ch == '\'':
			inSingleQuote = true
		case ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if inSingleQuote {
		return nil, false
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, true
}

func quoteArg(arg string) string {
	if arg == "" {
		return "''"
	}
	if strings.ContainsAny(arg, " \t\n\r'\"\\$`!&|;<>*?()[]{}") {
		return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
	}
	return arg
}
