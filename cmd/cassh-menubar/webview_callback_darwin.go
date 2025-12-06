//go:build darwin

package main

import "C"
import "log"

//export handleCasshURL
func handleCasshURL(urlCString *C.char) {
	urlString := C.GoString(urlCString)
	log.Printf("WebView handleCasshURL called with: %s", urlString)

	// Handle the URL in a goroutine to avoid blocking the main thread
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC in handleCasshURL: %v", r)
			}
		}()

		// Use the existing URL handler from urlhandler_darwin.go
		handleReceivedURL(urlString)
	}()
}
