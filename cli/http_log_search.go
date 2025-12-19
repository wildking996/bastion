package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// parseHTTPLogSearchArgs parses `http search` arguments into query params and page number.
//
// Supported flags:
// - --local-port <port> / --local-port=<port>
// - --bastion <name> / --bastion=<name>
// - --url <url> / --url=<url>
//
// The last argument may be a page number.
func parseHTTPLogSearchArgs(args []string) (page int, values url.Values, err error) {
	page = 1
	values = url.Values{}

	if len(args) == 0 {
		return 0, nil, fmt.Errorf("missing args")
	}

	// Optional trailing page number.
	if p, parseErr := strconv.Atoi(args[len(args)-1]); parseErr == nil {
		page = p
		args = args[:len(args)-1]
	}
	if page < 1 {
		page = 1
	}

	var keywordParts []string
	for i := 0; i < len(args); i++ {
		token := args[i]

		// --flag=value
		if strings.HasPrefix(token, "--") && strings.Contains(token, "=") {
			parts := strings.SplitN(token, "=", 2)
			flagName := parts[0]
			flagValue := strings.TrimSpace(parts[1])
			if err := applyHTTPLogSearchFlag(values, flagName, flagValue); err != nil {
				return 0, nil, err
			}
			continue
		}

		// --flag value
		if strings.HasPrefix(token, "--") {
			flagName := token
			if i+1 >= len(args) {
				return 0, nil, fmt.Errorf("missing value for %s", flagName)
			}
			flagValue := strings.TrimSpace(args[i+1])
			i++
			if err := applyHTTPLogSearchFlag(values, flagName, flagValue); err != nil {
				return 0, nil, err
			}
			continue
		}

		keywordParts = append(keywordParts, token)
	}

	if keyword := strings.TrimSpace(strings.Join(keywordParts, " ")); keyword != "" {
		values.Set("q", keyword)
	}

	if values.Get("q") == "" && values.Get("local_port") == "" && values.Get("bastion") == "" && values.Get("url") == "" {
		return 0, nil, fmt.Errorf("missing search keyword or filters")
	}

	return page, values, nil
}

func applyHTTPLogSearchFlag(values url.Values, name, value string) error {
	switch name {
	case "--local-port":
		p, err := strconv.Atoi(value)
		if err != nil || p <= 0 || p > 65535 {
			return fmt.Errorf("invalid --local-port: %q", value)
		}
		values.Set("local_port", strconv.Itoa(p))
		return nil
	case "--bastion":
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("invalid --bastion: empty")
		}
		values.Set("bastion", value)
		return nil
	case "--url":
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("invalid --url: empty")
		}
		values.Set("url", value)
		return nil
	default:
		return fmt.Errorf("unknown flag: %s", name)
	}
}
