// Package browser launches the user's default web browser.
package browser

// Open opens url in the default browser.
func Open(url string) error { return open(url) }
