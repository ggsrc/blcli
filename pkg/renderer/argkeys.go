package renderer

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Argument names in args files and default.yaml are usually kebab-case
// (hosted-domain) or snake_case (project_id), while templates read them in
// PascalCase (.HostedDomain, .ProjectID) or lowerCamelCase (.hostedDomain,
// .rootUrl, .argocdURL). The helpers below make one argument readable under
// all of these spellings.

// argKeyAbbreviations are words that PascalCase spellings keep in upper case,
// for example "project-id" -> "ProjectID".
var argKeyAbbreviations = map[string]string{
	"id":     "ID",
	"url":    "URL",
	"api":    "API",
	"uid":    "UID",
	"gid":    "GID",
	"pid":    "PID",
	"ip":     "IP",
	"dns":    "DNS",
	"tls":    "TLS",
	"ssl":    "SSL",
	"http":   "HTTP",
	"https":  "HTTPS",
	"json":   "JSON",
	"yaml":   "YAML",
	"xml":    "XML",
	"csv":    "CSV",
	"jwt":    "JWT",
	"oauth":  "OAuth",
	"argocd": "ArgoCD",
	"saml":   "SAML",
	"aws":    "AWS",
	"gcp":    "GCP",
	"azure":  "Azure",
	"k8s":    "K8s",
	"gke":    "GKE",
	"eks":    "EKS",
	"aks":    "AKS",
}

func splitArgKey(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_'
	})
}

func upperFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// PascalCaseArgKey converts kebab-case or snake_case to PascalCase and keeps
// common abbreviations in upper case: "project-id" -> "ProjectID",
// "argocd-url" -> "ArgoCDURL", "imageTag" -> "ImageTag".
func PascalCaseArgKey(s string) string {
	var b strings.Builder
	for _, part := range splitArgKey(s) {
		if abbrev, ok := argKeyAbbreviations[strings.ToLower(part)]; ok {
			b.WriteString(abbrev)
		} else {
			b.WriteString(upperFirst(part))
		}
	}
	return b.String()
}

// lowerCamelArgKey converts kebab-case or snake_case to lowerCamelCase. The
// first word is lower-cased; with abbrev, later words use the abbreviation
// table ("root-url" -> "rootURL"), otherwise only their first letter is
// upper-cased ("root-url" -> "rootUrl").
func lowerCamelArgKey(s string, abbrev bool) string {
	parts := splitArgKey(s)
	var b strings.Builder
	for i, part := range parts {
		switch {
		case i == 0:
			b.WriteString(strings.ToLower(part))
		case abbrev:
			if a, ok := argKeyAbbreviations[strings.ToLower(part)]; ok {
				b.WriteString(a)
				continue
			}
			b.WriteString(upperFirst(part))
		default:
			b.WriteString(upperFirst(part))
		}
	}
	return b.String()
}

// ArgKeyAliases returns the spellings, other than key itself, under which a
// template can read the argument key:
//
//	hosted-domain -> HostedDomain, hostedDomain
//	root-url      -> RootURL, rootUrl, rootURL
//	project_id    -> ProjectID, projectId, projectID
//	imageTag      -> ImageTag
//	IstioVersion  -> istioVersion
//
// A key without '-' or '_' gets its other first-letter case, except keys that
// start with an acronym (RBACPolicies, HPA), which are returned unchanged.
func ArgKeyAliases(key string) []string {
	var candidates []string
	if strings.ContainsAny(key, "-_") {
		candidates = []string{
			PascalCaseArgKey(key),
			lowerCamelArgKey(key, false),
			lowerCamelArgKey(key, true),
		}
	} else {
		r, size := utf8.DecodeRuneInString(key)
		switch {
		case r == utf8.RuneError:
		case unicode.IsLower(r):
			candidates = []string{upperFirst(key)}
		case unicode.IsUpper(r):
			next, _ := utf8.DecodeRuneInString(key[size:])
			if !unicode.IsUpper(next) {
				candidates = []string{string(unicode.ToLower(r)) + key[size:]}
			}
		}
	}

	seen := map[string]bool{key: true}
	aliases := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		aliases = append(aliases, c)
	}
	return aliases
}

// CanonicalArgKey returns the form that every spelling of one argument shares:
// lower case without '-' and '_'. hosted-domain, hostedDomain, HostedDomain and
// hosted_domain all map to "hosteddomain".
func CanonicalArgKey(key string) string {
	return strings.ToLower(strings.NewReplacer("-", "", "_", "").Replace(key))
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// AddArgAliases copies params into dst, each under its own key and under every
// alias from ArgKeyAliases. A key spelled out in params always keeps its own
// value; an alias never replaces a key of params, and when two keys of params
// share an alias the first one in sorted order wins. Aliases do replace keys
// that dst already had from elsewhere (for example flattened globals), because
// params are the more specific source.
func AddArgAliases(dst map[string]interface{}, params map[string]interface{}) {
	if dst == nil || len(params) == 0 {
		return
	}
	keys := sortedKeys(params)
	for _, k := range keys {
		dst[k] = params[k]
	}
	written := make(map[string]bool)
	for _, k := range keys {
		for _, alias := range ArgKeyAliases(k) {
			if _, explicit := params[alias]; explicit || written[alias] {
				continue
			}
			dst[alias] = params[k]
			written[alias] = true
		}
	}
}

// WithArgAliases returns a copy of params in which every argument is also
// readable under its aliases (see AddArgAliases).
func WithArgAliases(params map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(params)*3)
	AddArgAliases(out, params)
	return out
}

// MergeArgDefaults fills in defaults for the arguments that args does not set.
// Arguments are matched by CanonicalArgKey, so an args value given under any
// spelling (hosted-domain) replaces the default declared under another
// (hostedDomain), and every spelling of the default, including its aliases,
// then carries the args value. Keys already in args are never changed.
func MergeArgDefaults(args map[string]interface{}, defaults map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{}, len(args)+len(defaults)*3)
	for k, v := range args {
		merged[k] = v
	}

	given := make(map[string]interface{})
	for _, k := range sortedKeys(args) {
		c := CanonicalArgKey(k)
		if _, ok := given[c]; !ok {
			given[c] = args[k]
		}
	}

	for _, k := range sortedKeys(defaults) {
		spellings := append([]string{k}, ArgKeyAliases(k)...)
		for _, s := range spellings {
			if _, exists := merged[s]; exists {
				continue
			}
			if v, ok := given[CanonicalArgKey(s)]; ok {
				merged[s] = v
			} else {
				merged[s] = defaults[k]
			}
		}
	}
	return merged
}
