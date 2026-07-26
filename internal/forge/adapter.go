package forge

import (
	"os"
	"os/exec"
	"strings"
)

// Adapter descreve como interagir com um forge específico para abrir um PR/MR.
type Adapter struct {
	Forge     string
	Noun      string   // "Pull Request" ou "Merge Request"
	CLIName   string   // nome do executável (vazio = sem CLI)
	CLIArgs   []string // argumentos para criar PR/MR
	Available bool     // false quando CLI ausente ou TRACKFW_DISABLE_EXTERNAL_COMMANDS=1
}

// defaultAvailFn verifica se um comando existe no PATH, respeitando
// TRACKFW_DISABLE_EXTERNAL_COMMANDS (mesmo padrão de discover.go).
func defaultAvailFn(name string) bool {
	if name == "" {
		return false
	}
	if os.Getenv("TRACKFW_DISABLE_EXTERNAL_COMMANDS") == "1" {
		return false
	}
	_, err := exec.LookPath(name)
	return err == nil
}

// NewAdapter retorna o Adapter para o forge informado.
//
// availFn injeta a função de verificação de disponibilidade de CLI — útil em
// testes para evitar dependência de binários reais no PATH. Quando nil, usa
// exec.LookPath com respeito a TRACKFW_DISABLE_EXTERNAL_COMMANDS.
func NewAdapter(forge string, availFn func(string) bool) Adapter {
	if availFn == nil {
		availFn = defaultAvailFn
	}
	switch forge {
	case "github":
		return Adapter{
			Forge:     "github",
			Noun:      "Pull Request",
			CLIName:   "gh",
			CLIArgs:   []string{"pr", "create"},
			Available: availFn("gh"),
		}
	case "gitlab":
		return Adapter{
			Forge:     "gitlab",
			Noun:      "Merge Request",
			CLIName:   "glab",
			CLIArgs:   []string{"mr", "create"},
			Available: availFn("glab"),
		}
	case "azure":
		return Adapter{
			Forge:     "azure",
			Noun:      "Pull Request",
			CLIName:   "az",
			CLIArgs:   []string{"repos", "pr", "create"},
			Available: availFn("az"),
		}
	case "bitbucket":
		// Bitbucket não possui CLI oficial — sempre usa URL de fallback.
		// Nunca chama availFn.
		return Adapter{
			Forge:     "bitbucket",
			Noun:      "Pull Request",
			CLIName:   "",
			CLIArgs:   nil,
			Available: false,
		}
	default:
		return Adapter{
			Forge:     forge,
			Noun:      "Pull Request",
			CLIName:   "",
			CLIArgs:   nil,
			Available: false,
		}
	}
}

// FallbackURL retorna a URL para abrir o PR/MR no browser dado um remote URL
// e o nome do branch de origem.
func (a Adapter) FallbackURL(remoteURL, branch string) string {
	base := remoteHTTPSBase(remoteURL, a.Forge)
	if base == "" {
		return ""
	}
	switch a.Forge {
	case "github":
		return base + "/compare/" + branch + "?expand=1"
	case "gitlab":
		return base + "/-/merge_requests/new?merge_request[source_branch]=" + branch
	case "bitbucket":
		return base + "/pull-requests/new?source=" + branch
	case "azure":
		return base + "/pullrequestcreate?sourceRef=" + branch
	}
	return ""
}

// remoteHTTPSBase converte qualquer formato de URL de remote git (git@, ssh://,
// https://, http://) para a URL base HTTPS sem .git.
//
// Para Azure SSH (host ssh.dev.azure.com) aplica normalização:
//
//	git@ssh.dev.azure.com:v3/org/project/repo → https://dev.azure.com/org/project/_git/repo
func remoteHTTPSBase(rawURL, forge string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	var host, pathStr string

	switch {
	case strings.HasPrefix(rawURL, "git@"):
		// git@host:path
		rest := rawURL[4:]
		colonIdx := strings.IndexByte(rest, ':')
		if colonIdx < 0 {
			return ""
		}
		host = strings.ToLower(rest[:colonIdx])
		pathStr = rest[colonIdx+1:]

	case strings.HasPrefix(rawURL, "ssh://"):
		// ssh://[user@]host/path
		rest := rawURL[6:]
		if atIdx := strings.IndexByte(rest, '@'); atIdx >= 0 {
			rest = rest[atIdx+1:]
		}
		slashIdx := strings.IndexByte(rest, '/')
		if slashIdx < 0 {
			host = strings.ToLower(rest)
		} else {
			host = strings.ToLower(rest[:slashIdx])
			pathStr = rest[slashIdx+1:]
		}

	case strings.HasPrefix(rawURL, "https://") || strings.HasPrefix(rawURL, "http://"):
		rawURL = strings.TrimPrefix(rawURL, "https://")
		rawURL = strings.TrimPrefix(rawURL, "http://")
		// remover user@
		if atIdx := strings.IndexByte(rawURL, '@'); atIdx >= 0 {
			rawURL = rawURL[atIdx+1:]
		}
		slashIdx := strings.IndexByte(rawURL, '/')
		if slashIdx < 0 {
			host = strings.ToLower(rawURL)
		} else {
			host = strings.ToLower(rawURL[:slashIdx])
			pathStr = rawURL[slashIdx+1:]
		}

	default:
		return ""
	}

	pathStr = strings.TrimSuffix(pathStr, ".git")
	pathStr = strings.Trim(pathStr, "/")

	// Normalização especial para Azure SSH (ssh.dev.azure.com → dev.azure.com)
	if forge == "azure" && host == "ssh.dev.azure.com" {
		host = "dev.azure.com"
		pathStr = strings.TrimPrefix(pathStr, "v3/")
		// v3/org/project/repo → org/project/_git/repo
		parts := strings.SplitN(pathStr, "/", 3)
		if len(parts) == 3 {
			pathStr = parts[0] + "/" + parts[1] + "/_git/" + parts[2]
		}
	}

	if pathStr == "" {
		return "https://" + host
	}
	return "https://" + host + "/" + pathStr
}
