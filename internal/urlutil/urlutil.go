// Package urlutil provides string utilities.
package urlutil

import "net/url"

// JoinPath joins the given path elements into a single path, separating them with slashes.
func JoinPath(parts ...string) string {
	// Ignore the error because we know the base === "/" is valid.
	resultURL, _ := url.JoinPath("/", parts...)
	return resultURL
}
