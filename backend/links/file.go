// Package links holds the quick links: bookmarked sites in named groups, kept in a file in the
// repo so they sync between machines through git, plus a cache of each site's favicon.
package links

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// QuickLinkGroup is one named grid of links, shown as a tab. gqlgen binds it to the GraphQL
// type of the same name.
type QuickLinkGroup struct {
	Name  string
	Links []*QuickLink
}

// QuickLink is one bookmarked site. Icon optionally overrides the site's favicon with an
// emoji (or any short text) or an image URL.
type QuickLink struct {
	Title string
	URL   string
	Icon  string
}

// FaviconURL is where the dashboard serves this site's favicon (see Favicons).
func (l *QuickLink) FaviconURL() string {
	origin, err := Origin(l.URL)
	if err != nil {
		return ""
	}
	return "/icons?site=" + url.QueryEscape(origin)
}

// QuickLinkGroupInput and QuickLinkInput are the saveQuickLinks mutation's arguments.
type QuickLinkGroupInput struct {
	Name  string
	Links []*QuickLinkInput
}

type QuickLinkInput struct {
	Title string
	URL   string
	Icon  *string
}

// defaultGroup holds links that appear in the file before any [group] header.
const defaultGroup = "links"

const fileHeader = `# Quick links, shown in the dashboard's Quick links widget and editable there.
# Groups start with [name]. Each link is "Title | URL", with an optional third field that
# replaces the site's icon with an emoji or an image URL: "Title | URL | 🎮".
`

// Parse reads a links file. Lines it can't use are skipped and described in problems.
func Parse(r io.Reader) (groups []*QuickLinkGroup, problems []string) {
	var current *QuickLinkGroup
	group := func(name string) *QuickLinkGroup {
		for _, g := range groups {
			if strings.EqualFold(g.Name, name) {
				return g
			}
		}
		g := &QuickLinkGroup{Name: name, Links: []*QuickLink{}}
		groups = append(groups, g)
		return g
	}

	scanner := bufio.NewScanner(r)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := cleanName(line[1 : len(line)-1])
			if name == "" {
				problems = append(problems, fmt.Sprintf("line %d: group name is empty", n))
				continue
			}
			current = group(name)
			continue
		}

		fields := strings.SplitN(line, "|", 3)
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		var title, rawURL, icon string
		switch len(fields) {
		case 1: // a bare URL
			rawURL = fields[0]
		case 2:
			title, rawURL = fields[0], fields[1]
		default:
			title, rawURL, icon = fields[0], fields[1], fields[2]
		}
		link, err := newLink(title, rawURL, icon)
		if err != nil {
			problems = append(problems, fmt.Sprintf("line %d: %v", n, err))
			continue
		}
		if current == nil {
			current = group(defaultGroup)
		}
		current.Links = append(current.Links, link)
	}
	return groups, problems
}

// Format writes groups in the file format Parse reads.
func Format(groups []*QuickLinkGroup) []byte {
	var b bytes.Buffer
	b.WriteString(fileHeader)
	for _, g := range groups {
		fmt.Fprintf(&b, "\n[%s]\n", g.Name)
		for _, l := range g.Links {
			if l.Icon != "" {
				fmt.Fprintf(&b, "%s | %s | %s\n", l.Title, l.URL, l.Icon)
			} else {
				fmt.Fprintf(&b, "%s | %s\n", l.Title, l.URL)
			}
		}
	}
	return b.Bytes()
}

// FromInput validates and tidies links from the editor: names and titles are trimmed, URLs
// without a scheme get https://, and a missing title becomes the site's host name.
func FromInput(in []*QuickLinkGroupInput) ([]*QuickLinkGroup, error) {
	groups := make([]*QuickLinkGroup, 0, len(in))
	seen := map[string]bool{}
	for _, gi := range in {
		name := cleanName(gi.Name)
		if name == "" {
			return nil, errors.New("group name can't be empty")
		}
		if seen[strings.ToLower(name)] {
			return nil, fmt.Errorf("there are two groups named %q", name)
		}
		seen[strings.ToLower(name)] = true

		g := &QuickLinkGroup{Name: name, Links: []*QuickLink{}}
		for _, li := range gi.Links {
			icon := ""
			if li.Icon != nil {
				icon = *li.Icon
			}
			link, err := newLink(li.Title, li.URL, icon)
			if err != nil {
				return nil, err
			}
			g.Links = append(g.Links, link)
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func newLink(title, rawURL, icon string) (*QuickLink, error) {
	u, err := normalizeURL(rawURL)
	if err != nil {
		return nil, err
	}
	title = cleanField(title)
	if title == "" {
		parsed, _ := url.Parse(u)
		title = strings.TrimPrefix(parsed.Hostname(), "www.")
	}
	return &QuickLink{Title: title, URL: u, Icon: cleanField(icon)}, nil
}

// normalizeURL accepts http(s) URLs, adding https:// when there's no scheme.
func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("link URL can't be empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("%q isn't a web address", raw)
	}
	if strings.ContainsAny(raw, "| \t") {
		return "", fmt.Errorf("%q has spaces or | in it", raw)
	}
	return u.String(), nil
}

// Origin is a URL's scheme and host (with port), e.g. "https://github.com". Favicons are
// fetched and cached per origin.
func Origin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("%q isn't a web address", raw)
	}
	return strings.ToLower(u.Scheme + "://" + u.Host), nil
}

// cleanField keeps a title or icon on one line and out of the way of the | separators.
func cleanField(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(strings.ReplaceAll(s, "|", "/"))
}

func cleanName(s string) string {
	return strings.NewReplacer("[", "", "]", "").Replace(cleanField(s))
}
